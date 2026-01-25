package worker

import (
	"encoding/json"
	"fmt"
)

func parseBody(body string) (MessageDTO, error) {
	var dto MessageDTO
	if err := json.Unmarshal([]byte(body), &dto); err != nil {
		return MessageDTO{}, PermanentError{Err: fmt.Errorf("invalid json body: %w", err)}
	}
	return dto, nil
}
