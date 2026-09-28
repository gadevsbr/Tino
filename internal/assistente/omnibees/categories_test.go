package omnibees

import "testing"

func TestSemanticCategoriesPreservePhysicalSources(t *testing.T) {
	prices := map[string]int64{}
	for _, name := range roomNames {
		prices[normalize(name)] = 10000
	}
	for _, tc := range []struct {
		adults int
		ages   []int
	}{
		{2, nil}, {2, []int{5}}, {2, []int{5, 6}}, {3, []int{5, 6}},
	} {
		got := Categories(Search{Adults: tc.adults, Children: len(tc.ages), Ages: tc.ages, Discount: 5}, prices)
		if len(got) != len(roomNames) {
			t.Fatal(got)
		}
		for i, c := range got {
			if c.SourceKey == "" || c.Key == "" || c.TotalCents != 9500 {
				t.Fatalf("category %d: %+v", i, c)
			}
		}
	}
	if len(CategoryDefinitions()) != 6 {
		t.Fatal("catalog should need only six mappings")
	}
}

func TestUnavailableCategoryNeverOffered(t *testing.T) {
	prices := map[string]int64{normalize(roomNames["duplo"]): 12550, normalize(roomNames["superluxo"]): 0, normalize(roomNames["triploDeluxe"]): -1}
	got := Categories(Search{Adults: 2}, prices)
	if len(got) != 1 || got[0].Key != "interna" || got[0].TotalCents != 12550 {
		t.Fatal(got)
	}
	if got := Categories(Search{Adults: 4}, prices); len(got) != 1 || got[0].SourceKey != "duplo" {
		t.Fatal("OmniBees availability was incorrectly filtered")
	}
}
