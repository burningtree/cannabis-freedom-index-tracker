package research

import "testing"

func TestFigureGapsPointsOutMissingCounts(t *testing.T) {
	r := valid()
	r.Summary = "Detectives found 52 cannabis plants and 1.5 kg of dried cannabis in an attic."
	gaps := FigureGaps(r)
	if len(gaps) != 2 {
		t.Fatal(gaps)
	}
	plants, kg := 52, 1.5
	r.Figures = &Figures{PlantsDestroyed: &plants, KgSeized: &kg}
	if gaps = FigureGaps(r); len(gaps) != 0 {
		t.Fatal(gaps)
	}
	r.Outcome = "not_relevant"
	r.Figures = nil
	if FigureGaps(r) != nil {
		t.Fatal("gaps for an answer that is not a report")
	}
}
