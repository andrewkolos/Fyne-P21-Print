package imaging

import (
	"strings"
	"testing"
)

func TestCableWrapRepeatsAroundCable(t *testing.T) {
	img, info, err := RenderCableLabel(CableLabel{Text: "NAS", Mode: CableWrap, DiameterMM: 5, LabelLenMM: 40}, 96, 284)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 96 || b.Dy() != 284 {
		t.Fatalf("size %v", b)
	}
	// 5 mm cable: 15.7 mm around, 3 copies -> 5.2 mm pitch -> 6 copies on 35.5 mm.
	if !strings.Contains(info, "5.2 mm") {
		t.Errorf("info %q", info)
	}
}

func TestCableFlagTooThick(t *testing.T) {
	if _, _, err := RenderCableLabel(CableLabel{Text: "x", Mode: CableFlag, DiameterMM: 12, LabelLenMM: 40}, 96, 284); err == nil {
		t.Error("12 mm cable on a 40 mm label cannot leave two 6 mm flags")
	}
	if _, _, err := RenderCableLabel(CableLabel{Text: "x", Mode: CableFlag, DiameterMM: 5, LabelLenMM: 40}, 96, 284); err != nil {
		t.Error(err)
	}
}
