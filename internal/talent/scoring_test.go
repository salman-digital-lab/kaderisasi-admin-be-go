package talent

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func uniform(value int) []int {
	answers := make([]int, QuestionCount)
	for i := range answers {
		answers[i] = value
	}
	return answers
}
func TestDefinitionAndParticipantPrivacy(t *testing.T) {
	if err := ValidateDefinition(definition); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, question := range definition.Questions {
		if question.Reconstructed {
			count++
		}
	}
	if count != 28 {
		t.Fatalf("reconstructed=%d", count)
	}
	if definition.Questions[10].Talent != "Positivity" || definition.Questions[11].Talent != "Maximizer" {
		t.Fatal("exceptional mapping lost")
	}
	body, err := json.Marshal(PublicDefinition())
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"talent", "reconstructed", "domain"} {
		if strings.Contains(string(body), `"`+key+`"`) {
			t.Fatal("scoring key exposed")
		}
	}
}
func TestScoresAndDomainSummaries(t *testing.T) {
	for _, value := range []int{1, 6} {
		scores, err := Score(uniform(value))
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range scores.Talents {
			if row.Score != (value-1)*20 || row.Total != value*5 {
				t.Fatal(row)
			}
		}
		for _, row := range scores.Domains {
			if row.Score != float64((value-1)*20) {
				t.Fatal(row)
			}
		}
	}
	answers := uniform(1)
	for i, id := range []int{1, 35, 69, 103, 137} {
		answers[id-1] = []int{6, 5, 5, 6, 4}[i]
	}
	scores, err := Score(answers)
	if err != nil {
		t.Fatal(err)
	}
	if scores.Talents[0].Name != "Communication" || scores.Talents[0].Score != 84 {
		t.Fatal(scores.Talents[0])
	}
	counts := 0
	for _, row := range scores.Domains {
		counts += row.TopSevenCount
		if row.Name == "Pengaruh" && math.Abs(row.Score-10.5) > .001 {
			t.Fatal(row)
		}
	}
	if counts != 7 || scores.Talents[6].Group != "Bakat menonjol" || scores.Talents[7].Group != "Bakat pendukung" || scores.Talents[27].Group != "Bakat paling lemah" {
		t.Fatal("ranking groups/domain counts")
	}
}
func TestTieBreaks(t *testing.T) {
	answers := uniform(1)
	set := func(themeIndex int, values []int) {
		for i, id := range definition.Talents[themeIndex].Questions {
			answers[id-1] = values[i]
		}
	}
	set(0, []int{5, 5, 4, 2, 2})
	set(1, []int{6, 6, 2, 2, 2})
	scores, _ := Score(answers)
	if scores.Talents[0].Name != "Empathy" || !scores.Talents[0].EqualScore {
		t.Fatal("six count tie-break")
	}
	set(0, []int{5, 4, 4, 3, 2})
	set(1, []int{5, 5, 3, 3, 2})
	scores, _ = Score(answers)
	if scores.Talents[0].Name != "Empathy" {
		t.Fatal("five count tie-break")
	}
	scores, _ = Score(uniform(1))
	for i, row := range scores.Talents {
		if row.Name != definition.Talents[i].Name || !row.EqualScore {
			t.Fatal("stable final tie-break")
		}
	}
}
func TestIncompleteAndInvalidAnswers(t *testing.T) {
	for _, answers := range [][]int{nil, {1}, uniform(0), uniform(7), uniform(-1)} {
		if _, err := Score(answers); err == nil {
			t.Fatal("invalid answers accepted")
		}
	}
	if err := ValidateAnswers(uniform(0), false); err != nil {
		t.Fatal("blank draft rejected")
	}
}
