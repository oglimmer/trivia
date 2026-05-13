package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// SuggestRequest is what the frontend sends.
type SuggestRequest struct {
	// Hint is a free-form description from the user, e.g. "this is a picture of my dog".
	Hint string `json:"hint"`
	// AnswerType is one of yesno|choice|number.
	AnswerType string `json:"answerType"`
	// Optionally include the base64-encoded image (PNG/JPEG) so the model can see it.
	PhotoB64 string `json:"photoB64,omitempty"`
}

// SuggestResponse mirrors the question structure the user is filling in.
type SuggestResponse struct {
	Text    string   `json:"text"`
	Options []string `json:"options,omitempty"`
	Correct any      `json:"correct"`
}

// defaultBaseURL is the Anthropic Messages API root. Tests override BaseURL
// to point at an httptest server.
const defaultBaseURL = "https://api.anthropic.com"

type Client struct {
	APIKey  string
	Model   string
	BaseURL string
	HTTP    *http.Client
}

func New() *Client {
	model := os.Getenv("ANTHROPIC_MODEL")
	if model == "" {
		model = "claude-sonnet-4-6"
	}
	return &Client{
		APIKey:  os.Getenv("ANTHROPIC_API_KEY"),
		Model:   model,
		BaseURL: defaultBaseURL,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

var systemPrompt = `You write ONE genuinely surprising trivia question inspired by a photo the user uploaded.

Return a strict JSON object with these fields and nothing else (no prose, no code fences):
{ "text": string, "options": string[], "correct": any }

Rules by answerType:
- yesno    : options=["Yes","No"]; correct="yes" or "no".
- choice   : options=exactly 4 short answers; correct=integer index (0-based) of the right option.
- number   : options=[]; correct is the numeric answer (no units in the number itself).

What makes a GOOD trivia question here:
- It teaches the player something they almost certainly don't already know. A well-informed adult should pause and think — not answer instantly.
- Use the photo/hint as a springboard into a weird fact, historical oddity, record, etymology, science quirk, or cultural footnote about the subject. Do NOT just describe what's visible ("What color is the car?" is forbidden).
- Prefer specific, checkable facts over opinions or vibes. The "correct" answer must be objectively true and verifiable.
- A playful, witty, slightly cheeky tone is welcome — quirky and funky beats dry and textbook. But the FACT itself must be real; do not invent trivia to sound funny.
- For "choice", make wrong options plausible-but-wrong (close numbers, sibling species, neighboring countries, similar-sounding names) so guessing is hard.
- For "number", pick facts where the magnitude is itself surprising (counts, years, distances, speeds, weights).
- For "yesno", lean on counter-intuitive truths where the gut answer is wrong.

Hard rules:
- Keep "text" under ~140 chars, a single sentence, no preamble like "Did you know".
- Never reveal the answer inside the question.
- If the photo/hint is too vague to anchor a real fact, pick the most specific identifiable thing in it (object, place, species, brand, era) and build trivia around that.`

type anthropicReq struct {
	Model     string           `json:"model"`
	MaxTokens int              `json:"max_tokens"`
	System    string           `json:"system"`
	Messages  []map[string]any `json:"messages"`
}

type anthropicResp struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *Client) Suggest(ctx context.Context, req SuggestRequest) (*SuggestResponse, error) {
	if c.APIKey == "" {
		return nil, errors.New("ANTHROPIC_API_KEY not configured")
	}
	if req.AnswerType != "yesno" && req.AnswerType != "choice" && req.AnswerType != "number" {
		return nil, fmt.Errorf("invalid answerType: %s", req.AnswerType)
	}

	userBlocks := []map[string]any{}
	if req.PhotoB64 != "" {
		// Trim a leading data: URI if present.
		mediaType, b64 := splitDataURI(req.PhotoB64)
		userBlocks = append(userBlocks, map[string]any{
			"type": "image",
			"source": map[string]any{
				"type":       "base64",
				"media_type": mediaType,
				"data":       b64,
			},
		})
	}
	userBlocks = append(userBlocks, map[string]any{
		"type": "text",
		"text": fmt.Sprintf("answerType=%s\nhint=%s\nWrite the question now.", req.AnswerType, req.Hint),
	})

	body := anthropicReq{
		Model:     c.Model,
		MaxTokens: 512,
		System:    systemPrompt,
		Messages: []map[string]any{
			{"role": "user", "content": userBlocks},
		},
	}
	buf, _ := json.Marshal(body)

	base := c.BaseURL
	if base == "" {
		base = defaultBaseURL
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", base+"/v1/messages", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.HTTP.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("anthropic %d: %s", resp.StatusCode, string(rb))
	}
	var ar anthropicResp
	if err := json.Unmarshal(rb, &ar); err != nil {
		return nil, err
	}
	if ar.Error != nil {
		return nil, errors.New(ar.Error.Message)
	}
	if len(ar.Content) == 0 {
		return nil, errors.New("empty response")
	}
	text := ar.Content[0].Text
	// Be permissive: extract first {...} block in case the model wrapped output.
	start := strings.IndexByte(text, '{')
	end := strings.LastIndexByte(text, '}')
	if start < 0 || end <= start {
		return nil, fmt.Errorf("could not parse JSON from: %s", text)
	}
	out := &SuggestResponse{}
	if err := json.Unmarshal([]byte(text[start:end+1]), out); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w; raw=%s", err, text)
	}
	return out, nil
}

// splitDataURI separates a data: URI's media type from its base64 payload.
// A plain base64 string passes through with the default JPEG media type.
func splitDataURI(s string) (mediaType, data string) {
	mediaType = "image/jpeg"
	if !strings.HasPrefix(s, "data:") {
		return mediaType, s
	}
	semi := strings.IndexByte(s, ';')
	comma := strings.IndexByte(s, ',')
	if semi <= 5 || comma <= semi {
		return mediaType, s
	}
	return s[5:semi], s[comma+1:]
}
