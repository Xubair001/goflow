package job

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestHandlerFunc_Execute(t *testing.T) {
	called := false
	var gotPayload json.RawMessage
	h := HandlerFunc(func(_ context.Context, payload json.RawMessage) (json.RawMessage, error) {
		called = true
		gotPayload = payload
		return json.RawMessage(`{"ok":true}`), nil
	})

	result, err := h.Execute(context.Background(), json.RawMessage(`{"x":1}`))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !called {
		t.Fatal("Execute() did not invoke the underlying function")
	}
	if string(gotPayload) != `{"x":1}` {
		t.Errorf("payload = %s, want %s", gotPayload, `{"x":1}`)
	}
	if string(result) != `{"ok":true}` {
		t.Errorf("result = %s, want %s", result, `{"ok":true}`)
	}
}

func TestRegistry_RegisterAndLookup(t *testing.T) {
	r := NewRegistry()
	h := HandlerFunc(func(_ context.Context, p json.RawMessage) (json.RawMessage, error) { return p, nil })
	r.Register("send_email", h)

	got, err := r.Lookup("send_email")
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if got == nil {
		t.Fatal("Lookup() returned a nil handler for a registered type")
	}
}

func TestRegistry_LookupUnregistered(t *testing.T) {
	r := NewRegistry()
	_, err := r.Lookup("does_not_exist")
	if !errors.Is(err, ErrHandlerNotRegistered) {
		t.Errorf("Lookup() error = %v, want wrapping %v", err, ErrHandlerNotRegistered)
	}
}

func TestRegistry_RegisterDuplicatePanics(t *testing.T) {
	r := NewRegistry()
	noop := HandlerFunc(func(_ context.Context, p json.RawMessage) (json.RawMessage, error) { return p, nil })
	r.Register("send_email", noop)

	defer func() {
		if recover() == nil {
			t.Fatal("Register() did not panic on duplicate registration")
		}
	}()
	r.Register("send_email", noop)
}

func TestRegistry_Types(t *testing.T) {
	r := NewRegistry()
	noop := HandlerFunc(func(_ context.Context, p json.RawMessage) (json.RawMessage, error) { return p, nil })
	r.Register("send_email", noop)
	r.Register("resize_image", noop)

	types := r.Types()
	if len(types) != 2 {
		t.Fatalf("Types() returned %d entries, want 2: %v", len(types), types)
	}
	want := map[string]bool{"send_email": true, "resize_image": true}
	for _, ty := range types {
		if !want[ty] {
			t.Errorf("Types() returned unexpected type %q", ty)
		}
	}
}
