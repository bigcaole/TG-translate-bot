package translator

import (
	"context"
	"fmt"

	translate "cloud.google.com/go/translate/apiv3"
	"cloud.google.com/go/translate/apiv3/translatepb"
)

// Client wraps Google Cloud Translation API v3.
type Client struct {
	client     *translate.TranslationClient
	parentPath string
}

func NewClient(ctx context.Context, projectID, location string) (*Client, error) {
	cli, err := translate.NewTranslationClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create translation client: %w", err)
	}
	return &Client{
		client:     cli,
		parentPath: fmt.Sprintf("projects/%s/locations/%s", projectID, location),
	}, nil
}

func (c *Client) DetectLanguage(ctx context.Context, text string) (string, error) {
	resp, err := c.client.DetectLanguage(ctx, &translatepb.DetectLanguageRequest{
		Parent:   c.parentPath,
		MimeType: "text/plain",
		Source: &translatepb.DetectLanguageRequest_Content{
			Content: text,
		},
	})
	if err != nil {
		return "", fmt.Errorf("detect language: %w", err)
	}
	if len(resp.GetLanguages()) == 0 {
		return "", fmt.Errorf("detect language: empty result")
	}
	return resp.GetLanguages()[0].GetLanguageCode(), nil
}

func (c *Client) TranslateText(ctx context.Context, text, sourceLang, targetLang string) (string, error) {
	req := &translatepb.TranslateTextRequest{
		Parent:             c.parentPath,
		MimeType:           "text/plain",
		Contents:           []string{text},
		TargetLanguageCode: targetLang,
	}
	if sourceLang != "" {
		req.SourceLanguageCode = sourceLang
	}

	resp, err := c.client.TranslateText(ctx, req)
	if err != nil {
		return "", fmt.Errorf("translate text: %w", err)
	}
	if len(resp.GetTranslations()) == 0 {
		return "", fmt.Errorf("translate text: empty result")
	}
	return resp.GetTranslations()[0].GetTranslatedText(), nil
}

func (c *Client) Close() error {
	if c.client == nil {
		return nil
	}
	return c.client.Close()
}
