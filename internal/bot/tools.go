package bot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/swiftahul20/personal-messenger/internal/chat"
)

const (
	maxToolIterations = 3
	maxToolArgBytes   = 4096
	maxActiveTrivia   = 1000
	triviaLifetime    = 15 * time.Minute
)

const (
	toolStartTrivia  = "start_trivia"
	toolAnswerTrivia = "answer_trivia"
	toolGetWeather   = "get_weather"
)

type ToolExecutor interface {
	Execute(context.Context, int, string, string, json.RawMessage) (any, error)
}

type ToolCallingResponder interface {
	ReplyWithTools(context.Context, []chat.Message, int, string, ToolExecutor) (string, error)
}

type triviaQuestion struct {
	prompt  string
	answers []string
}

type triviaSession struct {
	question  triviaQuestion
	expiresAt time.Time
}

type ToolBox struct {
	mu       sync.Mutex
	sessions map[int]triviaSession
	weather  *WeatherClient
	now      func() time.Time
}

func NewToolBox(client *http.Client) *ToolBox {
	return &ToolBox{
		sessions: make(map[int]triviaSession),
		weather:  newWeatherClient(client),
		now:      time.Now,
	}
}

func (t *ToolBox) Execute(ctx context.Context, userID int, userMessage, name string, raw json.RawMessage) (any, error) {
	switch name {
	case toolStartTrivia:
		if err := decodeToolArguments(raw, &struct{}{}); err != nil {
			return nil, err
		}
		return t.startTrivia(userID)
	case toolAnswerTrivia:
		if err := decodeToolArguments(raw, &struct{}{}); err != nil {
			return nil, err
		}
		return t.answerTrivia(userID, userMessage)
	case toolGetWeather:
		var args struct {
			Location string `json:"location"`
		}
		if err := decodeToolArguments(raw, &args); err != nil {
			return nil, err
		}
		if !containsPhrase(userMessage, args.Location) {
			return nil, errors.New("location must be named in the user's message")
		}
		return t.weather.Current(ctx, args.Location)
	default:
		return nil, fmt.Errorf("tool %q is not available", name)
	}
}

func (t *ToolBox) startTrivia(userID int) (map[string]string, error) {
	now := t.now()
	t.mu.Lock()
	defer t.mu.Unlock()

	for id, session := range t.sessions {
		if !now.Before(session.expiresAt) {
			delete(t.sessions, id)
		}
	}
	if _, exists := t.sessions[userID]; !exists && len(t.sessions) >= maxActiveTrivia {
		return nil, errors.New("the trivia game limit has been reached")
	}

	question := triviaQuestions[(triviaIndex.Add(1)-1)%uint64(len(triviaQuestions))]
	t.sessions[userID] = triviaSession{
		question:  question,
		expiresAt: now.Add(triviaLifetime),
	}
	return map[string]string{
		"question": question.prompt,
		"message":  "Ask the user this question and wait for their answer.",
	}, nil
}

func (t *ToolBox) answerTrivia(userID int, userMessage string) (map[string]any, error) {
	now := t.now()
	t.mu.Lock()
	defer t.mu.Unlock()

	session, exists := t.sessions[userID]
	if !exists || !now.Before(session.expiresAt) {
		delete(t.sessions, userID)
		return nil, errors.New("there is no active trivia question")
	}
	for _, accepted := range session.question.answers {
		if containsPhrase(userMessage, accepted) {
			delete(t.sessions, userID)
			return map[string]any{
				"correct": true,
				"answer":  accepted,
				"message": "The user answered correctly.",
			}, nil
		}
	}
	return map[string]any{
		"correct": false,
		"message": "The answer is incorrect. Let the user guess again without revealing the answer.",
	}, nil
}

func containsPhrase(text, phrase string) bool {
	normalize := func(value string) string {
		words := strings.FieldsFunc(strings.ToLower(value), func(char rune) bool {
			return !unicode.IsLetter(char) && !unicode.IsDigit(char)
		})
		return strings.Join(words, " ")
	}
	text = " " + normalize(text) + " "
	phrase = " " + normalize(phrase) + " "
	return phrase != "  " && strings.Contains(text, phrase)
}

func decodeToolArguments(raw json.RawMessage, target any) error {
	if len(raw) == 0 {
		return errors.New("tool arguments are required")
	}
	if len(raw) > maxToolArgBytes {
		return errors.New("tool arguments exceed the size limit")
	}
	if trimmed := strings.TrimSpace(string(raw)); !strings.HasPrefix(trimmed, "{") {
		return errors.New("tool arguments must be a JSON object")
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid tool arguments: %w", err)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return errors.New("tool arguments must contain one JSON value")
	}
	return nil
}

func hasControlCharacters(value string) bool {
	for _, char := range value {
		if unicode.IsControl(char) {
			return true
		}
	}
	return false
}

var triviaIndex atomic.Uint64

var triviaQuestions = []triviaQuestion{
	{prompt: "What is the largest planet in our solar system?", answers: []string{"jupiter"}},
	{prompt: "How many sides does a hexagon have?", answers: []string{"6", "six"}},
	{prompt: "What is the capital city of Japan?", answers: []string{"tokyo"}},
	{prompt: "Which gas do plants absorb from the atmosphere?", answers: []string{"carbon dioxide", "co2"}},
	{prompt: "What is the chemical symbol for gold?", answers: []string{"au"}},
	{prompt: "Which ocean is the largest on Earth?", answers: []string{"pacific", "pacific ocean"}},
	{prompt: "How many colors are traditionally named in a rainbow?", answers: []string{"7", "seven"}},
	{prompt: "What is the closest star to Earth?", answers: []string{"the sun", "sun"}},
}
