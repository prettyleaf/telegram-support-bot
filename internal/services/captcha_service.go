package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	mathrand "math/rand"
	"strings"
	"sync"
	"time"

	"remnafamily/internal/config"
)

// CaptchaChallenge is a single onboarding verification challenge.
type CaptchaChallenge struct {
	SessionID    string
	Type         string
	Question     string
	Options      []string
	CorrectIndex int
	ExpiresAt    time.Time
}

// CaptchaService generates onboarding challenges for private users.
type CaptchaService struct {
	mu  sync.Mutex
	rng *mathrand.Rand
}

// NewCaptchaService creates a captcha generator service.
func NewCaptchaService() *CaptchaService {
	return &CaptchaService{
		rng: mathrand.New(mathrand.NewSource(time.Now().UnixNano())),
	}
}

// GenerateChallenge creates a new captcha challenge based on YAML config.
func (s *CaptchaService) GenerateChallenge(cfg config.CaptchaConfig) (*CaptchaChallenge, error) {
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}

	switch cfg.Type {
	case "button":
		return s.generateButtonChallenge(timeout), nil
	case "text":
		return s.generateTextChallenge(timeout), nil
	case "custom":
		return s.generateCustomChallenge(cfg.Questions, timeout)
	case "math":
		fallthrough
	default:
		return s.generateMathChallenge(timeout), nil
	}
}

func (s *CaptchaService) generateMathChallenge(timeout time.Duration) *CaptchaChallenge {
	operations := []string{"+", "-", "x"}
	op := operations[s.intn(len(operations))]

	var a, b, result int
	switch op {
	case "+":
		a = s.intn(20) + 1
		b = s.intn(20) + 1
		result = a + b
	case "-":
		a = s.intn(30) + 10
		b = s.intn(a)
		result = a - b
	default:
		a = s.intn(10) + 1
		b = s.intn(10) + 1
		result = a * b
	}

	options := []string{fmt.Sprintf("%d", result)}
	used := map[string]struct{}{options[0]: {}}
	for len(options) < 6 {
		wrong := result + s.intn(21) - 10
		if wrong < 0 {
			wrong = s.intn(50)
		}
		option := fmt.Sprintf("%d", wrong)
		if _, exists := used[option]; exists {
			continue
		}
		used[option] = struct{}{}
		options = append(options, option)
	}

	s.shuffle(options)
	correctIndex := indexOf(options, fmt.Sprintf("%d", result))

	return &CaptchaChallenge{
		SessionID:    randomSessionID(),
		Type:         "math",
		Question:     fmt.Sprintf("Solve the example / Решите пример: <b>%d %s %d = ?</b>", a, op, b),
		Options:      options,
		CorrectIndex: correctIndex,
		ExpiresAt:    time.Now().Add(timeout),
	}
}

func (s *CaptchaService) generateButtonChallenge(timeout time.Duration) *CaptchaChallenge {
	emojis := []string{"🍎", "🍊", "🍋", "🍌", "🍉", "🍇", "🍓", "🍒", "🍑", "🥝", "🍍", "🥥", "🥑", "🍆", "🥕"}
	correct := emojis[s.intn(len(emojis))]

	options := []string{correct}
	used := map[string]struct{}{correct: {}}
	for len(options) < 6 {
		option := emojis[s.intn(len(emojis))]
		if _, exists := used[option]; exists {
			continue
		}
		used[option] = struct{}{}
		options = append(options, option)
	}

	s.shuffle(options)

	return &CaptchaChallenge{
		SessionID:    randomSessionID(),
		Type:         "button",
		Question:     fmt.Sprintf("Tap the button with / Нажмите кнопку с эмодзи: <b>%s</b>", correct),
		Options:      options,
		CorrectIndex: indexOf(options, correct),
		ExpiresAt:    time.Now().Add(timeout),
	}
}

func (s *CaptchaService) generateTextChallenge(timeout time.Duration) *CaptchaChallenge {
	words := []string{
		"hello", "support", "ticket", "device", "panel",
		"captcha", "verify", "answer", "button", "message",
	}

	correct := words[s.intn(len(words))]
	options := []string{correct}
	used := map[string]struct{}{correct: {}}
	for len(options) < 6 {
		option := words[s.intn(len(words))]
		if _, exists := used[option]; exists {
			continue
		}
		used[option] = struct{}{}
		options = append(options, option)
	}

	s.shuffle(options)

	return &CaptchaChallenge{
		SessionID:    randomSessionID(),
		Type:         "text",
		Question:     fmt.Sprintf("Choose the word / Выберите слово: <b>%s</b>", strings.ToUpper(correct)),
		Options:      options,
		CorrectIndex: indexOf(options, correct),
		ExpiresAt:    time.Now().Add(timeout),
	}
}

func (s *CaptchaService) generateCustomChallenge(questions []config.CaptchaQuestionConfig, timeout time.Duration) (*CaptchaChallenge, error) {
	if len(questions) == 0 {
		return nil, fmt.Errorf("custom captcha has no questions configured")
	}

	question := questions[s.intn(len(questions))]
	options := append([]string(nil), question.Options...)
	s.shuffle(options)

	return &CaptchaChallenge{
		SessionID:    randomSessionID(),
		Type:         "custom",
		Question:     question.Question,
		Options:      options,
		CorrectIndex: indexOfFold(options, question.CorrectAnswer),
		ExpiresAt:    time.Now().Add(timeout),
	}, nil
}

func (s *CaptchaService) intn(limit int) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rng.Intn(limit)
}

func (s *CaptchaService) shuffle(values []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rng.Shuffle(len(values), func(i, j int) {
		values[i], values[j] = values[j], values[i]
	})
}

func randomSessionID() string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func indexOf(values []string, target string) int {
	for i, value := range values {
		if value == target {
			return i
		}
	}
	return 0
}

func indexOfFold(values []string, target string) int {
	for i, value := range values {
		if strings.EqualFold(value, target) {
			return i
		}
	}
	return 0
}
