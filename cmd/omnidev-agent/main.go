package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/zayeagle/omnidev-agent/internal/agent"
	"github.com/zayeagle/omnidev-agent/internal/config"
	"github.com/zayeagle/omnidev-agent/internal/llm"
	"github.com/zayeagle/omnidev-agent/internal/permissions"
	"github.com/zayeagle/omnidev-agent/internal/session"
	"github.com/zayeagle/omnidev-agent/internal/tools"
)

var version = "dev"

func main() {
	flags := parseFlags()

	if flags.version {
		fmt.Printf("omnidev-agent version %s  go%s/%s\n", version, runtime.Version(), runtime.GOARCH)
		os.Exit(0)
	}

	cfg, err := config.LoadWithLayers(config.LoadOptions{
		ProjectConfigPath: ".omnidev-agent.json",
		GlobalConfigPath:  os.ExpandEnv("$HOME/.omnidev-agent/config.json"),
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

	sess := session.New()
	sessionStore := session.NewStore(cfg.RuntimeSessionDir())

	cpStore := agent.NewCheckpointStore(".ai_history/checkpoints/")

	a := agent.New(provider, permChecker, toolbox, sess)
	a.SetMaxTurns(cfg.MaxTurns)
	a.SetStore(sessionStore)
	a.SetCheckpointStore(cpStore)
	if cfg.ContextMaxTokens > 0 {
		a.SetContextManager(agent.NewContextManager(provider, cfg.ContextMaxTokens, cfg.ContextSummarizeThreshold, ""))
	}

	cwd, _ := os.Getwd()
	guard := agent.NewProjectAwarenessGuard(toolbox, sess, cwd)
	a.SetGuard(guard)

	classifier := agent.NewClassifier(provider)
	a.SetClassifier(classifier)

	complexityClassifier := agent.NewComplexityClassifier(provider)
	a.SetComplexityClassifier(complexityClassifier)

	dispatcher := agent.NewTaskDispatcher(a)
	dispatcher.SetCheckpointStore(cpStore)
	a.SetDispatcher(dispatcher)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintf(os.Stderr, "\nSaving session...\n")
		sessionStore.Save(sess)
		sessionStore.Export(sess)
		fmt.Fprintf(os.Stderr, "Interrupted. Checkpoint preserved.\n")
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
	runTUI(a, cfg, guard)
}
