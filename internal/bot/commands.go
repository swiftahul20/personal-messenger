package bot

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

const (
	maxDiceCount = 100
	maxDiceSides = 1_000_000
)

var eightBallAnswers = [...]string{
	"yep, totally :D",
	"uh huh, for sure!",
	"looks good 2 me :)",
	"omg yes lol",
	"ask me again later, im busy :P",
	"hmm... not lookin great",
	"nah, prob not",
	"my sources say no lol",
}

func commandReply(content string, now time.Time) (string, bool) {
	fields := strings.Fields(content)
	if len(fields) == 0 || !strings.HasPrefix(fields[0], "/") {
		return "", false
	}

	switch strings.ToLower(fields[0]) {
	case "/help":
		return "try /help, /8ball [question], /roll [NdM], or /time. with Gemini, ask me for trivia or current weather. hit Buzz to shake a buddy's window :)", true
	case "/8ball":
		if len(fields) == 1 {
			return "ask me a yes/no question first, then try /8ball [question] :P", true
		}
		return eightBallAnswers[rand.Intn(len(eightBallAnswers))], true
	case "/roll":
		return rollReply(fields[1:])
	case "/time":
		return fmt.Sprintf("it's %s :)", now.Format("3:04 PM MST, Mon Jan 2")), true
	default:
		return "", false
	}
}

func rollReply(args []string) (string, bool) {
	if len(args) > 1 {
		return "usage: /roll [NdM], like /roll 2d6. try 1-100 dice with 2-1000000 sides.", true
	}

	spec := "1d6"
	if len(args) == 1 {
		spec = strings.ToLower(args[0])
	}
	parts := strings.Split(spec, "d")
	if len(parts) != 2 || parts[1] == "" {
		return "usage: /roll [NdM], like /roll 2d6. try 1-100 dice with 2-1000000 sides.", true
	}

	count := 1
	if parts[0] != "" {
		parsed, err := strconv.Atoi(parts[0])
		if err != nil {
			return "usage: /roll [NdM], like /roll 2d6. try 1-100 dice with 2-1000000 sides.", true
		}
		count = parsed
	}
	sides, err := strconv.Atoi(parts[1])
	if err != nil || count < 1 || count > maxDiceCount || sides < 2 || sides > maxDiceSides {
		return "usage: /roll [NdM], like /roll 2d6. try 1-100 dice with 2-1000000 sides.", true
	}

	rolls := make([]string, count)
	total := 0
	for i := range rolls {
		value := rand.Intn(sides) + 1
		total += value
		rolls[i] = strconv.Itoa(value)
	}
	return fmt.Sprintf("rolled %dd%d: %s = %d :D", count, sides, strings.Join(rolls, " + "), total), true
}
