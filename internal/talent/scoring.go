package talent

import (
	"embed"
	"encoding/json"
	"fmt"
	"slices"
)

//go:embed definition.json
var definitionFiles embed.FS

const QuestionCount = 170

type Question struct {
	ID            int    `json:"id"`
	Statement     string `json:"statement"`
	Talent        string `json:"talent"`
	Reconstructed bool   `json:"reconstructed"`
}
type Theme struct {
	Name      string `json:"name"`
	Domain    string `json:"domain"`
	Questions []int  `json:"questions"`
}
type Definition struct {
	Version   string     `json:"version"`
	Questions []Question `json:"questions"`
	Talents   []Theme    `json:"talents"`
}
type ParticipantQuestion struct {
	ID        int    `json:"id"`
	Statement string `json:"statement"`
}
type ParticipantDefinition struct {
	Version   string                `json:"version"`
	Questions []ParticipantQuestion `json:"questions"`
}

var definition = loadDefinition()

func loadDefinition() Definition {
	data, err := definitionFiles.ReadFile("definition.json")
	if err != nil {
		panic(err)
	}
	var result Definition
	if err = json.Unmarshal(data, &result); err != nil {
		panic(err)
	}
	if err = ValidateDefinition(result); err != nil {
		panic(err)
	}
	return result
}

func ValidateDefinition(d Definition) error {
	if d.Version == "" || len(d.Questions) != QuestionCount || len(d.Talents) != 34 {
		return fmt.Errorf("invalid assessment definition")
	}
	seen := make([]bool, QuestionCount)
	names := map[string]bool{}
	domains := []string{"Eksekusi", "Pengaruh", "Hubungan", "Pemikiran"}
	for i, q := range d.Questions {
		if q.ID != i+1 || q.Statement == "" {
			return fmt.Errorf("invalid question %d", i+1)
		}
	}
	for _, theme := range d.Talents {
		if names[theme.Name] || theme.Name == "" || !slices.Contains(domains, theme.Domain) || len(theme.Questions) != 5 {
			return fmt.Errorf("invalid theme %s", theme.Name)
		}
		names[theme.Name] = true
		for _, id := range theme.Questions {
			if id < 1 || id > QuestionCount || seen[id-1] || d.Questions[id-1].Talent != theme.Name {
				return fmt.Errorf("invalid scoring key %d", id)
			}
			seen[id-1] = true
		}
	}
	return nil
}

func PublicDefinition() ParticipantDefinition {
	questions := make([]ParticipantQuestion, len(definition.Questions))
	for i, q := range definition.Questions {
		questions[i] = ParticipantQuestion{q.ID, q.Statement}
	}
	return ParticipantDefinition{definition.Version, questions}
}

type TalentScore struct {
	Name       string `json:"name"`
	Domain     string `json:"domain"`
	Total      int    `json:"total"`
	Score      int    `json:"score"`
	Rank       int    `json:"rank"`
	Group      string `json:"group"`
	EqualScore bool   `json:"equal_score"`
	SixCount   int    `json:"-"`
	FiveCount  int    `json:"-"`
}
type DomainScore struct {
	Name          string  `json:"name"`
	Score         float64 `json:"score"`
	TopSevenCount int     `json:"top_seven_count"`
}
type Scores struct {
	Talents []TalentScore `json:"talents"`
	Domains []DomainScore `json:"domains"`
}

func ValidateAnswers(answers []int, complete bool) error {
	if len(answers) != QuestionCount {
		return fmt.Errorf("expected %d answers", QuestionCount)
	}
	for _, value := range answers {
		if value < 0 || value > 6 || (complete && value == 0) {
			return fmt.Errorf("answer outside permitted range")
		}
	}
	return nil
}

func Score(answers []int) (Scores, error) {
	if err := ValidateAnswers(answers, true); err != nil {
		return Scores{}, err
	}
	result := Scores{Talents: make([]TalentScore, 0, 34), Domains: []DomainScore{}}
	for _, theme := range definition.Talents {
		row := TalentScore{Name: theme.Name, Domain: theme.Domain}
		for _, id := range theme.Questions {
			value := answers[id-1]
			row.Total += value
			if value == 6 {
				row.SixCount++
			}
			if value == 5 {
				row.FiveCount++
			}
		}
		row.Score = (row.Total - 5) * 4
		result.Talents = append(result.Talents, row)
	}
	slices.SortStableFunc(result.Talents, func(a, b TalentScore) int {
		if a.Score != b.Score {
			return b.Score - a.Score
		}
		if a.SixCount != b.SixCount {
			return b.SixCount - a.SixCount
		}
		return b.FiveCount - a.FiveCount
	})
	for i := range result.Talents {
		row := &result.Talents[i]
		row.Rank = i + 1
		row.Group = "Bakat pendukung"
		if i < 7 {
			row.Group = "Bakat menonjol"
		} else if i >= 27 {
			row.Group = "Bakat paling lemah"
		}
		row.EqualScore = (i > 0 && result.Talents[i-1].Score == row.Score) || (i+1 < len(result.Talents) && result.Talents[i+1].Score == row.Score)
	}
	for _, name := range []string{"Eksekusi", "Pengaruh", "Hubungan", "Pemikiran"} {
		row := DomainScore{Name: name}
		count := 0
		for _, theme := range result.Talents {
			if theme.Domain == name {
				row.Score += float64(theme.Score)
				count++
				if theme.Rank <= 7 {
					row.TopSevenCount++
				}
			}
		}
		row.Score /= float64(count)
		result.Domains = append(result.Domains, row)
	}
	return result, nil
}
