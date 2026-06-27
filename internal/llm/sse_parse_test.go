package llm

import "testing"

func TestParseChatCompletionBody_JSON(t *testing.T) {
	body := []byte(`{"choices":[{"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}]}`)
	resp, err := parseChatCompletionBody(body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Choices[0].Message.Content == nil || *resp.Choices[0].Message.Content != "hi" {
		t.Fatalf("content = %v", resp.Choices[0].Message.Content)
	}
}

func TestParseChatCompletionBody_SSE(t *testing.T) {
	body := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hel\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n" +
		"data: [DONE]\n\n")
	resp, err := parseChatCompletionBody(body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Choices[0].Message.Content == nil || *resp.Choices[0].Message.Content != "Hello" {
		t.Fatalf("content = %v, want Hello", resp.Choices[0].Message.Content)
	}
}

func TestParseChatCompletionBody_PlainText(t *testing.T) {
	body := []byte("direct text reply")
	resp, err := parseChatCompletionBody(body)
	if err != nil {
		t.Fatal(err)
	}
	if *resp.Choices[0].Message.Content != "direct text reply" {
		t.Fatalf("content = %q", *resp.Choices[0].Message.Content)
	}
}
