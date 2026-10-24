package eventio

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/MiloSharp/Stoa-Inspect/internal/model"
)

const maxInputBytes = 64 << 20

func Read(reader io.Reader) ([]model.Event, error) {
	document, err := ReadDocument(reader)
	if err != nil {
		return nil, err
	}
	return document.Events, nil
}

type Document struct {
	Events    []model.Event
	Locations map[string]model.WorldLocation
}

func ReadDocument(reader io.Reader) (Document, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxInputBytes+1))
	if err != nil {
		return Document{}, fmt.Errorf("read input: %w", err)
	}
	if len(data) > maxInputBytes {
		return Document{}, fmt.Errorf("input exceeds %d MiB safety limit", maxInputBytes>>20)
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return Document{}, fmt.Errorf("input is empty")
	}
	switch trimmed[0] {
	case '[':
		events, err := readArray(trimmed)
		return Document{Events: events}, err
	case '{':
		var object map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &object); err == nil {
			if rawEvents, ok := object["events"]; ok {
				events, err := readArray(rawEvents)
				if err != nil {
					return Document{}, err
				}
				locations := map[string]model.WorldLocation{}
				if rawLocations, ok := object["locations"]; ok {
					if err := json.Unmarshal(rawLocations, &locations); err != nil {
						return Document{}, fmt.Errorf("decode world locations: %w", err)
					}
					for name, location := range locations {
						if strings.TrimSpace(location.Name) == "" {
							location.Name = name
							locations[name] = location
						}
					}
				}
				return Document{Events: events, Locations: locations}, nil
			}
			if event, err := decodeEvent(trimmed); err == nil {
				return Document{Events: []model.Event{event}}, nil
			}
		}
	}
	events, err := readNDJSON(trimmed)
	return Document{Events: events}, err
}

func readArray(data []byte) ([]model.Event, error) {
	var records []json.RawMessage
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("decode JSON event array: %w", err)
	}
	events := make([]model.Event, 0, len(records))
	for index, record := range records {
		event, err := decodeEvent(record)
		if err != nil {
			return nil, fmt.Errorf("decode event %d: %w", index+1, err)
		}
		events = append(events, event)
	}
	return events, nil
}

func readNDJSON(data []byte) ([]model.Event, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64*1024), 8<<20)
	events := make([]model.Event, 0)
	for line := 1; scanner.Scan(); line++ {
		record := bytes.TrimSpace(scanner.Bytes())
		if len(record) == 0 || bytes.HasPrefix(record, []byte("#")) {
			continue
		}
		event, err := decodeEvent(record)
		if err != nil {
			return nil, fmt.Errorf("decode NDJSON line %d: %w", line, err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan NDJSON: %w", err)
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("input contains no events")
	}
	return events, nil
}

func decodeEvent(record []byte) (model.Event, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(record, &fields); err != nil {
		return model.Event{}, err
	}
	if payload, ok := fields["payload"]; ok {
		var nested map[string]json.RawMessage
		if json.Unmarshal(payload, &nested) == nil {
			for key, value := range nested {
				if _, exists := fields[key]; !exists {
					fields[key] = value
				}
			}
		}
	}
	aliases := map[string]string{"event_id": "id", "event_type": "type", "simulated_at": "simulatedAt", "activity_id": "activityId", "proposal_id": "proposalId"}
	for alias, canonical := range aliases {
		if _, exists := fields[canonical]; !exists {
			if value, ok := fields[alias]; ok {
				fields[canonical] = value
			}
		}
	}
	normalized, err := json.Marshal(fields)
	if err != nil {
		return model.Event{}, err
	}
	var event model.Event
	if err := json.Unmarshal(normalized, &event); err != nil {
		return model.Event{}, err
	}
	event.Present = make(map[string]bool, len(fields))
	for key := range fields {
		event.Present[key] = true
	}
	event.Type = strings.TrimSpace(event.Type)
	event.ID = strings.TrimSpace(event.ID)
	event.Actor = strings.TrimSpace(event.Actor)
	return event, nil
}
