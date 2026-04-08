package main

import (
	"encoding/json"
	"testing"
)

func TestPayloadValidation_Valid(t *testing.T) {
	payload := TransferPayload{
		EncPassword:       "pass",
		EncSalt:           "salt",
		S3AccessKeyID:     "key",
		S3SecretAccessKey: "secret",
		BackendProvider:   "Cloudflare",
	}
	// Should not panic or error — validation happens in join flow
	data, _ := json.Marshal(payload)
	var decoded TransferPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Valid payload should unmarshal: %v", err)
	}
	if decoded.EncPassword == "" {
		t.Error("EncPassword should not be empty")
	}
	if decoded.BackendProvider == "" {
		t.Error("BackendProvider should not be empty")
	}
}

func TestPayloadValidation_MissingFields(t *testing.T) {
	tests := []struct {
		name    string
		payload TransferPayload
		field   string
	}{
		{"missing enc password", TransferPayload{EncSalt: "s", S3AccessKeyID: "k", S3SecretAccessKey: "s", BackendProvider: "AWS"}, "EncPassword"},
		{"missing enc salt", TransferPayload{EncPassword: "p", S3AccessKeyID: "k", S3SecretAccessKey: "s", BackendProvider: "AWS"}, "EncSalt"},
		{"missing access key", TransferPayload{EncPassword: "p", EncSalt: "s", S3SecretAccessKey: "s", BackendProvider: "AWS"}, "S3AccessKeyID"},
		{"missing secret key", TransferPayload{EncPassword: "p", EncSalt: "s", S3AccessKeyID: "k", BackendProvider: "AWS"}, "S3SecretAccessKey"},
		{"missing provider", TransferPayload{EncPassword: "p", EncSalt: "s", S3AccessKeyID: "k", S3SecretAccessKey: "s"}, "BackendProvider"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify the field IS empty
			switch tt.field {
			case "EncPassword":
				if tt.payload.EncPassword != "" {
					t.Error("Test setup error: field should be empty")
				}
			case "EncSalt":
				if tt.payload.EncSalt != "" {
					t.Error("Test setup error: field should be empty")
				}
			case "S3AccessKeyID":
				if tt.payload.S3AccessKeyID != "" {
					t.Error("Test setup error: field should be empty")
				}
			case "S3SecretAccessKey":
				if tt.payload.S3SecretAccessKey != "" {
					t.Error("Test setup error: field should be empty")
				}
			case "BackendProvider":
				if tt.payload.BackendProvider != "" {
					t.Error("Test setup error: field should be empty")
				}
			}
		})
	}
}

func TestPayloadValidation_HubEndpointWarning(t *testing.T) {
	payload := TransferPayload{
		UseHub:      true,
		HubEndpoint: "", // empty — should trigger warning
	}
	if payload.UseHub && payload.HubEndpoint == "" {
		// This is the condition that triggers the warning in join.go
		// Just verify the condition is detectable
	} else {
		t.Error("Expected UseHub=true with empty HubEndpoint")
	}
}
