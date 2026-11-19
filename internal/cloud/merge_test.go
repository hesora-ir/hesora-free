package cloud

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestMergeDocumentDataCombinesIndependentFieldsAndReportsSameFieldConflict(t *testing.T) {
	base := json.RawMessage(`{"Name":"Buyer","Phone":"111","Address":"old","Nested":{"A":1,"B":1},"Items":[{"Name":"old"}]}`)
	local := json.RawMessage(`{"Name":"Desktop name","Phone":"111","Address":"old","Nested":{"A":2,"B":1},"Items":[{"Name":"desktop"}]}`)
	remote := json.RawMessage(`{"Name":"Web name","Phone":"222","Address":"new","Nested":{"A":1,"B":3},"Items":[{"Name":"web"}],"NewServerField":"preserve"}`)

	merged, conflicts, err := MergeDocumentData(base, local, remote, true)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err = json.Unmarshal(merged, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"Name": "Desktop name", "Phone": "222", "Address": "new", "Nested": map[string]any{"A": float64(2), "B": float64(3)}, "Items": []any{map[string]any{"Name": "desktop"}}, "NewServerField": "preserve"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("merged=%s want=%v", merged, want)
	}
	if !reflect.DeepEqual(conflicts, []string{"Items", "Name"}) {
		t.Fatalf("conflicts=%v", conflicts)
	}

	merged, conflicts, err = MergeDocumentData(base, local, remote, false)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(conflicts, []string{"Items", "Name"}) {
		t.Fatalf("reported conflicts=%v", conflicts)
	}
	if err = json.Unmarshal(merged, &got); err != nil {
		t.Fatal(err)
	}
	if got["Name"] != "Web name" {
		t.Fatalf("remote-wins name=%v", got["Name"])
	}
}

func TestMergeDocumentDataAppliesFieldDeletionFromRemoteAndKeepsUnknownLocalOmissions(t *testing.T) {
	base := json.RawMessage(`{"Known":"same","Optional":"base"}`)
	local := json.RawMessage(`{"Known":"same"}`)
	remote := json.RawMessage(`{"Known":"same"}`)
	merged, conflicts, err := MergeDocumentData(base, local, remote, true)
	if err != nil || len(conflicts) != 0 {
		t.Fatalf("conflicts=%v err=%v", conflicts, err)
	}
	if string(merged) != `{"Known":"same"}` {
		t.Fatalf("merged deletion=%s", merged)
	}
}

func TestMergeDocumentDataRejectsMalformedOrMultipleJSONValues(t *testing.T) {
	for _, raw := range [][]byte{[]byte(`{`), []byte(`{} {}`)} {
		if _, _, err := MergeDocumentData(raw, []byte(`{}`), []byte(`{}`), true); err == nil {
			t.Fatalf("accepted invalid base JSON %q", raw)
		}
	}
}
