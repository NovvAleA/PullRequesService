package storage

// TestCase представляет один тестовый случай
type TestCase struct {
	name           string
	testType       string
	input          interface{}
	expectedResult interface{}
	wantError      bool
}

// PickRandomInput входные данные для тестирования pickRandomDistinct
type PickRandomInput struct {
	arr []string
	n   int
}

// testTable возвращает тестовые случаи для логических функций storage
func testTable() []TestCase {
	return []TestCase{
		{
			name:     "Pick random from array",
			testType: "PickRandomDistinct",
			input: PickRandomInput{
				arr: []string{"a", "b", "c", "d"},
				n:   2,
			},
			wantError: false,
		},
	}
}
