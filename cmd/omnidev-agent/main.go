package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/zayeagle/omnidev-agent/internal/agent"
	"github.com/zayeagle/omnidev-agent/internal/config"
	"github.com/zayeagle/omnidev-agent/internal/llm"
	"github.com/zayeagle/omnidev-agent/internal/mcp"
	"github.com/zayeagle/omnidev-agent/internal/permissions"
	"github.com/zayeagle/omnidev-agent/internal/session"
	"github.com/zayeagle/omnidev-agent/internal/skills"
	"github.com/zayeagle/omnidev-agent/internal/tools"
	verpkg "github.com/zayeagle/omnidev-agent/internal/version"
)

var (
	appVersion = "0.0.0"
	buildTime  = "unknown"
)

func main() {
	flags := parseFlags()

	if flags.version {
		fmt.Printf("omnidev-agent %s  built %s  go%s/%s\n", verpkg.Display(appVersion), buildTime, runtime.Version(), runtime.GOARCH)
		os.Exit(0)
	}

	cfg, err := config.LoadWithLayers(config.LoadOptions{
		ProjectConfigPath: ".omnidev-agent.json",
		GlobalConfigPath:  config.DefaultGlobalConfigPath(),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	if flags.provider != "" {
		cfg.Provider = flags.provider
	}
	if flags.model != "" {
		cfg.Model = flags.model
	}

	var provider llm.Provider
	provider = llm.NewProvider(cfg.Provider, cfg.BaseURL, cfg.APIKey, cfg.Model,
		llm.OptionsFromConfig(cfg.MaxTokens, cfg.Temperature, cfg.Timeout, cfg.CompatMode))

	permChecker := permissions.NewForRun(flags.prompt != "", flags.yolo)
	toolbox := tools.NewRegistry()
	tools.RegisterAll(toolbox)
	tools.SetResultLimits(cfg.ToolResultLimits())

	skillCat := skills.LoadCatalog(cfg.SkillsSearchDirs())
	tools.SetSkillCatalog(skillCat)
	tools.RegisterSkills(toolbox)

	mcpMgr, err := mcp.Start(context.Background(), cfg.MCPServerConfigs())
	if err != nil {
		fmt.Fprintf(os.Stderr, "mcp: %v\n", err)
	}
	if mcpMgr != nil {
		mcpMgr.RegisterTools(toolbox, cfg.MCPServerConfigs())
		defer mcpMgr.Close()
	}

	sess := session.New()
	sessionStore := session.NewStore(cfg.RuntimeSessionDir())
	if loaded, err := sessionStore.LoadActive(); err != nil {
		fmt.Fprintf(os.Stderr, "session load: %v\n", err)
	} else if loaded != nil && loaded.Count() > 0 {
		sess = loaded
	}

	cpStore := agent.NewCheckpointStore(".ai_history/checkpoints/")

	a := agent.New(provider, permChecker, toolbox, sess)
	a.SetMaxTurns(cfg.MaxTurns)
	retryCfg := cfg.LLMRetryConfig()
	a.SetRetryConfig(retryCfg)
	a.SetMaxConsecutiveDenials(cfg.EffectiveMaxConsecutiveToolDenials())
	a.SetPipelineOptions(cfg.PipelineOptions())
	a.SetContextSlimOptions(cfg.ContextSlimOptions())
	a.SetSkillCatalog(skillCat)
	if mcpMgr != nil {
		a.SetMCPManager(mcpMgr)
	}
	a.SetStore(sessionStore)
	a.SetCheckpointStore(cpStore)
	if cfg.ContextMaxTokens > 0 {
		cm := agent.NewContextManager(provider, cfg.EffectiveContextMaxTokens(), cfg.EffectiveContextSummarizeThreshold(), "")
		cm.SetRetryConfig(retryCfg)
		a.SetContextManager(cm)
	}

	cwd, _ := os.Getwd()
	guard := agent.NewProjectAwarenessGuard(toolbox, sess, cwd)
	guard.SetAnalysisMaxChars(cfg.ContextSlimOptions().GuardAnalysisMax)
	a.SetGuard(guard)

	classifier := agent.NewClassifier(provider)
	classifier.SetRetryConfig(retryCfg)
	a.SetClassifier(classifier)

	complexityClassifier := agent.NewComplexityClassifier(provider)
	complexityClassifier.SetRetryConfig(retryCfg)
	a.SetComplexityClassifier(complexityClassifier)

	dispatcher := agent.NewTaskDispatcher(a, agent.DispatcherOptions{
		MaxParallel:        cfg.MaxParallel,
		SubAgentTimeout:      time.Duration(cfg.SubAgentTimeout) * time.Second,
		SubAgentMaxTurns:     cfg.SubAgentMaxTurns,
		SubAgentMaxRetries:   cfg.SubAgentMaxRetries,
	})
	dispatcher.SetCheckpointStore(cpStore)
	a.SetDispatcher(dispatcher)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintf(os.Stderr, "\nSaving session...\n")
		_ = sessionStore.SaveActive(a.Session())
		fmt.Fprintf(os.Stderr, "Interrupted. Session preserved.\n")
		os.Exit(0)
	}()

	// ── Headless mode: -p "task" ──
	if flags.prompt != "" {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			<-sigCh
			cancel()
		}()

		err = runHeadless(ctx, a, sess, sessionStore, flags.prompt)
		if err != nil && !errors.Is(err, context.Canceled) {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// ── TUI mode (default) ──
	runTUI(a, cfg, guard, sessionStore)
}
