// Package sender provides HTTP sending functionality for WAL frame batches.
//
// This package implements multipart form upload of compressed frame batches
// to a remote service. It supports custom HTTP clients for testing and
// alternative transport mechanisms.
//
// # Usage
//
// Create an HTTP sender:
//
//	sender := sender.NewHTTPSender(httpClient, logger)
//
//	metadata := sender.SendMetadata{
//	    ChainID:    "my-chain",
//	    NodeID:     "node-1",
//	    AuthKey:    "api-key",
//	    ServiceURL: "https://api.example.com",
//	}
//
//	if err := sender.Send(ctx, batch, metadata); err != nil {
//	    return err
//	}
//
// # Custom Senders
//
// Implement the Sender interface to send to alternative destinations
// (e.g., Kafka, S3, local files).
//
// # Version
//
// Current version: 1.0.0
// Minimum compatible version: 1.0.0
//
// See version.go for version constants that can be used programmatically.
package sender
