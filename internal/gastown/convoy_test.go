package gastown

import (
	"errors"
	"testing"
)

func TestConvoyListHappy(t *testing.T) {
	defer mockRun([]byte(gtConvoyListJSON), nil)()
	convoys, err := ConvoyList()
	if err != nil {
		t.Fatalf("ConvoyList() error = %v", err)
	}
	if len(convoys) != 1 {
		t.Fatalf("expected 1 convoy, got %d", len(convoys))
	}
	if convoys[0].ID != "conv-001" {
		t.Errorf("ID = %q, want conv-001", convoys[0].ID)
	}
}

func TestConvoyListEmpty(t *testing.T) {
	defer mockRun([]byte(`[]`), nil)()
	convoys, err := ConvoyList()
	if err != nil {
		t.Fatalf("ConvoyList() error = %v", err)
	}
	if len(convoys) != 0 {
		t.Errorf("expected 0 convoys, got %d", len(convoys))
	}
}

func TestConvoyListExecError(t *testing.T) {
	defer mockRun(nil, errors.New("connection refused"))()
	_, err := ConvoyList()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestConvoyListMalformedJSON(t *testing.T) {
	defer mockRun([]byte(`{not json`), nil)()
	_, err := ConvoyList()
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

func TestConvoyStatusHappy(t *testing.T) {
	defer mockRun([]byte(gtConvoyStatusJSON), nil)()
	detail, err := ConvoyStatus("conv-001")
	if err != nil {
		t.Fatalf("ConvoyStatus() error = %v", err)
	}
	if detail.ID != "conv-001" {
		t.Errorf("ID = %q, want conv-001", detail.ID)
	}
	if detail.Completed != 1 || detail.Total != 3 {
		t.Errorf("progress = %d/%d, want 1/3", detail.Completed, detail.Total)
	}
}

func TestConvoyStatusExecError(t *testing.T) {
	defer mockRun(nil, errors.New("not found"))()
	_, err := ConvoyStatus("conv-999")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestConvoyCreateReturnsID(t *testing.T) {
	defer mockCombined([]byte("conv-002\n"), nil)()
	id, err := ConvoyCreate("test-convoy", []string{"mg-1", "mg-2"})
	if err != nil {
		t.Fatalf("ConvoyCreate() error = %v", err)
	}
	if id != "conv-002\n" {
		t.Errorf("ID = %q, want %q", id, "conv-002\n")
	}
}

func TestConvoyCreateExecError(t *testing.T) {
	defer mockCombined([]byte("error output"), errors.New("exit 1"))()
	_, err := ConvoyCreate("test", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestConvoyCreateFromEpicArgs(t *testing.T) {
	calls, restore := mockCombinedCapture([]byte("conv-003\n"), nil)
	defer restore()
	_, err := ConvoyCreateFromEpic("epic-convoy", "mg-100")
	if err != nil {
		t.Fatalf("ConvoyCreateFromEpic() error = %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(*calls))
	}
	args := (*calls)[0]
	// Should be: gt convoy create epic-convoy --from-epic mg-100
	wantArgs := []string{"gt", "convoy", "create", "epic-convoy", "--from-epic", "mg-100"}
	if len(args) != len(wantArgs) {
		t.Fatalf("args = %v, want %v", args, wantArgs)
	}
	for i, want := range wantArgs {
		if args[i] != want {
			t.Errorf("args[%d] = %q, want %q", i, args[i], want)
		}
	}
}

func TestConvoyAddArgs(t *testing.T) {
	calls, restore := mockCombinedCapture(nil, nil)
	defer restore()
	err := ConvoyAdd("conv-001", []string{"mg-20", "mg-21"})
	if err != nil {
		t.Fatalf("ConvoyAdd() error = %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(*calls))
	}
	args := (*calls)[0]
	// Should be: gt convoy add conv-001 mg-20 mg-21
	if len(args) != 6 || args[4] != "mg-20" || args[5] != "mg-21" {
		t.Errorf("args = %v", args)
	}
}

func TestConvoyCloseArgs(t *testing.T) {
	calls, restore := mockCombinedCapture(nil, nil)
	defer restore()
	err := ConvoyClose("conv-001")
	if err != nil {
		t.Fatalf("ConvoyClose() error = %v", err)
	}
	args := (*calls)[0]
	// Should be: gt convoy close conv-001
	if len(args) != 4 || args[2] != "close" || args[3] != "conv-001" {
		t.Errorf("args = %v", args)
	}
}

func TestConvoyLandArgs(t *testing.T) {
	calls, restore := mockCombinedCapture(nil, nil)
	defer restore()
	err := ConvoyLand("conv-001")
	if err != nil {
		t.Fatalf("ConvoyLand() error = %v", err)
	}
	args := (*calls)[0]
	// Should be: gt convoy land conv-001
	if len(args) != 4 || args[2] != "land" || args[3] != "conv-001" {
		t.Errorf("args = %v", args)
	}
}
