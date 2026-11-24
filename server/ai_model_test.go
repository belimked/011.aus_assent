package main

import "testing"

func TestAUBedrockModel(t *testing.T) {
	ok := []string{
		"au.anthropic.claude-haiku-4-5-20251001-v1:0",
		"arn:aws:bedrock:ap-southeast-2:731558755170:application-inference-profile/96diwvwepaie",
		"arn:aws:bedrock:ap-southeast-4:731558755170:application-inference-profile/abc123",
	}
	bad := []string{
		"",
		"anthropic.claude-haiku-4-5-20251001-v1:0",
		"us.anthropic.claude-haiku-4-5-20251001-v1:0",
		"global.anthropic.claude-haiku-4-5-20251001-v1:0",
		"arn:aws:bedrock:us-east-1:731558755170:application-inference-profile/96diwvwepaie",
		"arn:aws:bedrock:ap-southeast-2:731558755170:inference-profile/us.anthropic.claude-haiku-4-5-20251001-v1:0",
		"arn:aws:bedrock:ap-southeast-2:731558755170:application-inference-profile/96diwvwepaie/extra",
	}
	for _, m := range ok {
		if !auBedrockModel(m) {
			t.Errorf("want allowed: %s", m)
		}
	}
	for _, m := range bad {
		if auBedrockModel(m) {
			t.Errorf("want rejected: %s", m)
		}
	}
}
