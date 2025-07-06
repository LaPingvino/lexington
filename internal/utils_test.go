package internal

import (
	"reflect"
	"testing"
)

func TestFilter(t *testing.T) {
	tests := []struct {
		name      string
		input     []int
		predicate func(int) bool
		expected  []int
	}{
		{
			name:      "filter even numbers",
			input:     []int{1, 2, 3, 4, 5, 6},
			predicate: func(n int) bool { return n%2 == 0 },
			expected:  []int{2, 4, 6},
		},
		{
			name:      "filter greater than 3",
			input:     []int{1, 2, 3, 4, 5},
			predicate: func(n int) bool { return n > 3 },
			expected:  []int{4, 5},
		},
		{
			name:      "empty slice",
			input:     []int{},
			predicate: func(n int) bool { return true },
			expected:  []int{},
		},
		{
			name:      "no matches",
			input:     []int{1, 3, 5},
			predicate: func(n int) bool { return n%2 == 0 },
			expected:  []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Filter(tt.input, tt.predicate)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Filter() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMap(t *testing.T) {
	tests := []struct {
		name      string
		input     []int
		transform func(int) string
		expected  []string
	}{
		{
			name:      "int to string",
			input:     []int{1, 2, 3},
			transform: func(n int) string { return string(rune(n + '0')) },
			expected:  []string{"1", "2", "3"},
		},
		{
			name:      "empty slice",
			input:     []int{},
			transform: func(n int) string { return "x" },
			expected:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Map(tt.input, tt.transform)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Map() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFind(t *testing.T) {
	tests := []struct {
		name      string
		input     []string
		predicate func(string) bool
		expected  string
		found     bool
	}{
		{
			name:      "find existing element",
			input:     []string{"apple", "banana", "cherry"},
			predicate: func(s string) bool { return s == "banana" },
			expected:  "banana",
			found:     true,
		},
		{
			name:      "element not found",
			input:     []string{"apple", "banana", "cherry"},
			predicate: func(s string) bool { return s == "orange" },
			expected:  "",
			found:     false,
		},
		{
			name:      "empty slice",
			input:     []string{},
			predicate: func(s string) bool { return true },
			expected:  "",
			found:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, found := Find(tt.input, tt.predicate)
			if result != tt.expected || found != tt.found {
				t.Errorf("Find() = (%v, %v), want (%v, %v)", result, found, tt.expected, tt.found)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		value    string
		expected bool
	}{
		{
			name:     "contains element",
			input:    []string{"a", "b", "c"},
			value:    "b",
			expected: true,
		},
		{
			name:     "does not contain element",
			input:    []string{"a", "b", "c"},
			value:    "d",
			expected: false,
		},
		{
			name:     "empty slice",
			input:    []string{},
			value:    "a",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Contains(tt.input, tt.value)
			if result != tt.expected {
				t.Errorf("Contains() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestUnique(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		expected []int
	}{
		{
			name:     "with duplicates",
			input:    []int{1, 2, 2, 3, 1, 4},
			expected: []int{1, 2, 3, 4},
		},
		{
			name:     "no duplicates",
			input:    []int{1, 2, 3},
			expected: []int{1, 2, 3},
		},
		{
			name:     "empty slice",
			input:    []int{},
			expected: []int{},
		},
		{
			name:     "all same",
			input:    []int{1, 1, 1},
			expected: []int{1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Unique(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Unique() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGroupBy(t *testing.T) {
	type person struct {
		name string
		age  int
	}

	people := []person{
		{"Alice", 25},
		{"Bob", 30},
		{"Charlie", 25},
		{"David", 30},
	}

	result := GroupBy(people, func(p person) int { return p.age })

	expected := map[int][]person{
		25: {{"Alice", 25}, {"Charlie", 25}},
		30: {{"Bob", 30}, {"David", 30}},
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GroupBy() = %v, want %v", result, expected)
	}
}
