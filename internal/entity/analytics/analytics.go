package analytics

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"time"

	"github.com/m7medvision/go-clean-template/internal/entity"
)

const QueuePrefix = "analytics-"

var (
	ErrUnknownEvent = errors.New("unknown analytics event")
	ErrUnknownSink  = errors.New("unknown analytics sink")
	ErrRejected     = errors.New("analytics sink rejected the event")
)

type Properties interface {
	Name() string
}

type Event struct {
	ID         string
	DistinctID string
	At         time.Time
	Properties Properties
}

func Events() []Properties {
	return slices.Concat(userEvents(), taskEvents(), exchangeEvents())
}

func EventID(e *entity.RecordedEvent) string {
	return fmt.Sprintf("%s-%d", e.StreamID, e.StreamVersion)
}

type envelope struct {
	Name       string          `json:"name"`
	ID         string          `json:"id"`
	DistinctID string          `json:"distinct_id"`
	At         time.Time       `json:"at"`
	Properties json.RawMessage `json:"properties"`
}

func Encode(e *Event) ([]byte, error) {
	props, err := json.Marshal(e.Properties)
	if err != nil {
		return nil, fmt.Errorf("analytics - Encode - json.Marshal: %w", err)
	}

	body, err := json.Marshal(envelope{
		Name: e.Properties.Name(), ID: e.ID, DistinctID: e.DistinctID, At: e.At, Properties: props,
	})
	if err != nil {
		return nil, fmt.Errorf("analytics - Encode - json.Marshal envelope: %w", err)
	}

	return body, nil
}

func Decode(body []byte) (*Event, error) {
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("analytics - Decode - json.Unmarshal envelope: %w", err)
	}

	i := slices.IndexFunc(Events(), func(p Properties) bool { return p.Name() == env.Name })
	if i < 0 {
		return nil, fmt.Errorf("analytics - Decode: %w: %q", ErrUnknownEvent, env.Name)
	}

	v := reflect.New(reflect.TypeOf(Events()[i]))
	if err := json.Unmarshal(env.Properties, v.Interface()); err != nil {
		return nil, fmt.Errorf("analytics - Decode - json.Unmarshal %s: %w", env.Name, err)
	}

	props, ok := v.Elem().Interface().(Properties)
	if !ok {
		return nil, fmt.Errorf("analytics - Decode: %w: %q", ErrUnknownEvent, env.Name)
	}

	return &Event{ID: env.ID, DistinctID: env.DistinctID, At: env.At, Properties: props}, nil
}
