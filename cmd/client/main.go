package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type shipment struct {
	ID              string  `json:"id"`
	ReferenceNumber string  `json:"reference_number"`
	Origin          string  `json:"origin"`
	Destination     string  `json:"destination"`
	Status          string  `json:"status"`
	ShipmentAmount  float64 `json:"shipment_amount"`
}

type event struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Comment   string `json:"comment"`
	CreatedAt string `json:"created_at"`
}

func main() {
	base := flag.String("addr", "http://localhost:8080", "REST API base URL")
	flag.Parse()

	api := *base + "/api/v1"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := &http.Client{}
	fmt.Printf("Connected to %s\n\n", *base)

	// 1. Create a shipment
	fmt.Println("=== POST /shipments ===")
	var created shipment
	doRequest(ctx, client, http.MethodPost, api+"/shipments", map[string]any{
		"reference_number": "SHP-2026-001",
		"origin":           "New York, NY",
		"destination":      "Los Angeles, CA",
		"driver_name":      "John Smith",
		"unit_number":      "TRUCK-42",
		"shipment_amount":  5000.00,
		"driver_revenue":   1500.00,
	}, http.StatusCreated, &created)
	fmt.Printf("Created shipment: id=%s ref=%s status=%s\n\n",
		created.ID, created.ReferenceNumber, created.Status)

	shipmentID := created.ID

	// 2. Get the shipment
	fmt.Println("=== GET /shipments/{id} ===")
	var got shipment
	doRequest(ctx, client, http.MethodGet, api+"/shipments/"+shipmentID, nil, http.StatusOK, &got)
	fmt.Printf("Got shipment: id=%s origin=%s dest=%s status=%s amount=%.2f\n\n",
		got.ID, got.Origin, got.Destination, got.Status, got.ShipmentAmount)

	// 3. Walk through the lifecycle: picked_up -> in_transit -> delivered
	transitions := []struct {
		status  string
		comment string
	}{
		{"picked_up", "Driver picked up the shipment"},
		{"in_transit", "Shipment is en route"},
		{"delivered", "Shipment delivered to destination"},
	}

	for _, t := range transitions {
		fmt.Printf("=== POST /shipments/{id}/events: %s ===\n", t.status)
		var evt event
		doRequest(ctx, client, http.MethodPost, api+"/shipments/"+shipmentID+"/events", map[string]any{
			"status":  t.status,
			"comment": t.comment,
		}, http.StatusCreated, &evt)
		fmt.Printf("Event added: id=%s status=%s comment=%q\n\n", evt.ID, evt.Status, evt.Comment)
	}

	// 4. Try an invalid transition (delivered -> picked_up)
	fmt.Println("=== POST invalid transition (delivered -> picked_up) ===")
	status, body := rawRequest(ctx, client, http.MethodPost, api+"/shipments/"+shipmentID+"/events", map[string]any{
		"status":  "picked_up",
		"comment": "This should fail",
	})
	if status >= 400 {
		fmt.Printf("Expected error (HTTP %d): %s\n\n", status, body)
	} else {
		fmt.Println("ERROR: should have been rejected!")
		os.Exit(1)
	}

	// 5. Get full event history
	fmt.Println("=== GET /shipments/{id}/events ===")
	var events []event
	doRequest(ctx, client, http.MethodGet, api+"/shipments/"+shipmentID+"/events", nil, http.StatusOK, &events)
	for i, e := range events {
		fmt.Printf("  [%d] status=%s comment=%q time=%s\n", i+1, e.Status, e.Comment, e.CreatedAt)
	}
	fmt.Println()

	// 6. List all shipments
	fmt.Println("=== GET /shipments ===")
	var shipments []shipment
	doRequest(ctx, client, http.MethodGet, api+"/shipments", nil, http.StatusOK, &shipments)
	for _, sh := range shipments {
		fmt.Printf("  id=%s ref=%s status=%s\n", sh.ID, sh.ReferenceNumber, sh.Status)
	}

	fmt.Println("\nAll tests passed!")
}

// doRequest performs an HTTP request, asserts the expected status code, and decodes the body into out.
func doRequest(ctx context.Context, client *http.Client, method, url string, payload any, wantStatus int, out any) {
	status, body := rawRequest(ctx, client, method, url, payload)
	if status != wantStatus {
		log.Fatalf("%s %s: expected HTTP %d, got %d: %s", method, url, wantStatus, status, body)
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			log.Fatalf("%s %s: decode response: %v", method, url, err)
		}
	}
}

// rawRequest performs an HTTP request and returns the status code and raw response body.
func rawRequest(ctx context.Context, client *http.Client, method, url string, payload any) (int, []byte) {
	var reqBody io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			log.Fatalf("%s %s: marshal payload: %v", method, url, err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		log.Fatalf("%s %s: build request: %v", method, url, err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("%s %s: read response: %v", method, url, err)
	}

	return resp.StatusCode, body
}
