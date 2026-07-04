package presentation

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/teoritty/xqs-plugin-telnet/internal/domain"
)

var allowedFieldIDs = map[string]struct{}{
	domain.FieldUsername:       {},
	domain.FieldPassword:       {},
	domain.FieldAutoLogin:      {},
	domain.FieldTerminalType:   {},
	domain.FieldBinaryMode:     {},
	domain.FieldLoginPrompt:    {},
	domain.FieldPasswordPrompt: {},
	domain.FieldLoginDelayMs:   {},
}

type connectDTO struct {
	SessionID    string            `json:"sessionId"`
	ConnectionID string            `json:"connectionId"`
	Protocol     string            `json:"protocol"`
	Host         string            `json:"host"`
	Port         int               `json:"port"`
	Username     string            `json:"username"`
	Fields       map[string]string `json:"fields"`
}

// MapConnectDTO converts session.connect JSON params to domain.ConnectionConfig.
func MapConnectDTO(params json.RawMessage) (domain.ConnectionConfig, error) {
	var dto connectDTO
	if err := json.Unmarshal(params, &dto); err != nil {
		return domain.ConnectionConfig{}, fmt.Errorf("invalid connect params")
	}

	cfg := domain.ConnectionConfig{
		SessionID:    domain.SessionID(strings.TrimSpace(dto.SessionID)),
		ConnectionID: strings.TrimSpace(dto.ConnectionID),
		Protocol:     strings.TrimSpace(dto.Protocol),
		Host:         strings.TrimSpace(dto.Host),
		Port:         dto.Port,
		Username:     strings.TrimSpace(dto.Username),
		Fields:       sanitizeFields(dto.Fields),
	}

	if cfg.Protocol == "" {
		cfg.Protocol = "telnet"
	}
	if err := cfg.Validate(); err != nil {
		return domain.ConnectionConfig{}, err
	}
	return cfg, nil
}

func sanitizeFields(fields map[string]string) map[string]string {
	if len(fields) == 0 {
		return nil
	}
	out := make(map[string]string, len(fields))
	for k, v := range fields {
		if _, ok := allowedFieldIDs[k]; !ok {
			continue
		}
		out[k] = v
	}
	return out
}

type resizeDTO struct {
	Cols uint32 `json:"cols"`
	Rows uint32 `json:"rows"`
}

// MapResizeDTO parses session.resize notification params.
func MapResizeDTO(params json.RawMessage) (uint16, uint16, error) {
	var dto resizeDTO
	if err := json.Unmarshal(params, &dto); err != nil {
		return 0, 0, fmt.Errorf("invalid resize params")
	}
	cols := clampUint16(dto.Cols, 80)
	rows := clampUint16(dto.Rows, 24)
	return cols, rows, nil
}

type inputDTO struct {
	DataBase64 string `json:"dataBase64"`
}

// MapInputDTO parses session.writeInput notification params.
func MapInputDTO(params json.RawMessage) ([]byte, error) {
	var dto inputDTO
	if err := json.Unmarshal(params, &dto); err != nil {
		return nil, fmt.Errorf("invalid input params")
	}
	if dto.DataBase64 == "" {
		return nil, nil
	}
	return base64.StdEncoding.DecodeString(dto.DataBase64)
}

func clampUint16(v uint32, fallback uint16) uint16 {
	if v == 0 {
		return fallback
	}
	if v > 65535 {
		return 65535
	}
	return uint16(v)
}
