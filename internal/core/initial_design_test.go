package core

import (
	"encoding/json"
	"os"
	"testing"
)

func TestInitialInvitationContract(t *testing.T) {
	data, err := os.ReadFile("testdata/initial-invitations.json")
	if err != nil {
		t.Fatal(err)
	}
	var inputs []EventInput
	if err := json.Unmarshal(data, &inputs); err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 2 {
		t.Fatal("expected both event products")
	}
	for _, in := range inputs {
		t.Run(in.TemplateID, func(t *testing.T) {
			if err := ValidateEvent(in, true); err != nil {
				t.Fatal(err)
			}
			if ValidateEvent(in, false) == nil {
				t.Fatal("metadata edits must not replace design")
			}
			changed := in
			changed.Title = "Different event"
			if ValidateEvent(changed, true) == nil {
				t.Fatal("inconsistent metadata accepted")
			}
			changed = in
			design := *in.InitialDesign
			changed.InitialDesign = &design
			design.Sections = []Section{{ID: "invalid"}}
			if ValidateEvent(changed, true) == nil {
				t.Fatal("invalid canvas accepted")
			}
		})
	}
}
