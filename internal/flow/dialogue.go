package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gadevsbr/tino/internal/assistente/commercial"
	"strings"
	"sync/atomic"
	"time"
)

var CommercialAICalls atomic.Uint64

func (ai *AIService) Converse(ctx context.Context, request commercial.DialogueRequest) (commercial.DialogueAnswer, error) {
	if ai == nil {
		return commercial.DialogueAnswer{}, fmt.Errorf("IA desativada")
	}
	clone := *ai
	clone.Endpoint = strings.TrimSuffix(ai.Endpoint, "/reply") + "/conversation"
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	content, err := clone.Complete(ctx, request)
	if err != nil {
		return commercial.DialogueAnswer{}, err
	}
	var answer commercial.DialogueAnswer
	if err = json.Unmarshal([]byte(content), &answer); err != nil {
		return answer, fmt.Errorf("resposta de atendimento inválida")
	}
	CommercialAICalls.Add(1)
	return answer, nil
}
