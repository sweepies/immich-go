package gen

import "testing"

func TestDeleteItemRemovesMatchesWithoutMutatingInput(t *testing.T) {
	input := []int{1, 2, 3, 2, 4}
	got := DeleteItem(input, 2)

	if len(got) != 3 || got[0] != 1 || got[1] != 3 || got[2] != 4 {
		t.Fatalf("unexpected filtered values: %v", got)
	}
	if len(input) != 5 || input[0] != 1 || input[1] != 2 || input[2] != 3 || input[3] != 2 || input[4] != 4 {
		t.Fatalf("input slice was mutated: %v", input)
	}
}

func TestDeleteItemNilInputReturnsEmptySlice(t *testing.T) {
	var input []int
	got := DeleteItem(input, 1)

	if got == nil {
		t.Fatal("expected an empty non-nil slice")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty result, got %v", got)
	}
}

func TestMapKeysReturnsAllKeys(t *testing.T) {
	m := map[string]int{"a": 1, "b": 2, "c": 3}
	got := MapKeys(m)

	if len(got) != len(m) {
		t.Fatalf("expected %d keys, got %d", len(m), len(got))
	}
	seen := map[string]bool{}
	for _, k := range got {
		seen[k] = true
	}
	for k := range m {
		if !seen[k] {
			t.Fatalf("missing key %q in %v", k, got)
		}
	}
}

func TestMapKeysSorted(t *testing.T) {
	m := map[string]int{"z": 1, "b": 2, "a": 3}
	got := MapKeysSorted(m)

	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "z" {
		t.Fatalf("unexpected sorted keys: %v", got)
	}
}

func TestMapKeysEmptyMapReturnsEmptySlice(t *testing.T) {
	got := MapKeys(map[string]int{})
	if got == nil {
		t.Fatal("expected an empty non-nil slice")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty result, got %v", got)
	}
}
