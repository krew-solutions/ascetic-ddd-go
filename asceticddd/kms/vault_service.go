package kms

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session/rest"
)

var extractHttpClient = rest.ExtractHttpClient

type VaultTransitOption func(*VaultTransitService)

func WithMount(mount string) VaultTransitOption {
	return func(v *VaultTransitService) {
		v.mount = mount
	}
}

func WithKeyType(keyType string) VaultTransitOption {
	return func(v *VaultTransitService) {
		v.keyType = keyType
	}
}

type VaultTransitService struct {
	vaultAddr  string
	vaultToken string
	mount      string
	keyType    string
}

func NewVaultTransitService(vaultAddr, vaultToken string, opts ...VaultTransitOption) *VaultTransitService {
	v := &VaultTransitService{
		vaultAddr:  vaultAddr,
		vaultToken: vaultToken,
		mount:      "transit",
		keyType:    "aes256-gcm96",
	}
	for _, opt := range opts {
		opt(v)
	}
	return v
}

func (v *VaultTransitService) EncryptDek(s session.Session, tenantId any, dek []byte) ([]byte, error) {
	if err := v.ensureKey(s, tenantId); err != nil {
		return nil, err
	}
	result, err := v.request(s, http.MethodPost, "/encrypt/"+v.keyName(tenantId), map[string]any{
		"plaintext": base64.StdEncoding.EncodeToString(dek),
	})
	if err != nil {
		return nil, err
	}
	data := result["data"].(map[string]any)
	return []byte(data["ciphertext"].(string)), nil
}

func (v *VaultTransitService) DecryptDek(s session.Session, tenantId any, encryptedDek []byte) ([]byte, error) {
	result, err := v.request(s, http.MethodPost, "/decrypt/"+v.keyName(tenantId), map[string]any{
		"ciphertext": string(encryptedDek),
	})
	if err != nil {
		return nil, err
	}
	data := result["data"].(map[string]any)
	return base64.StdEncoding.DecodeString(data["plaintext"].(string))
}

func (v *VaultTransitService) GenerateDek(s session.Session, tenantId any) ([]byte, []byte, error) {
	if err := v.ensureKey(s, tenantId); err != nil {
		return nil, nil, err
	}
	result, err := v.request(s, http.MethodPost, "/datakey/plaintext/"+v.keyName(tenantId), map[string]any{
		"bits": 256,
	})
	if err != nil {
		return nil, nil, err
	}
	data := result["data"].(map[string]any)
	plaintextDek, err := base64.StdEncoding.DecodeString(data["plaintext"].(string))
	if err != nil {
		return nil, nil, err
	}
	encryptedDek := []byte(data["ciphertext"].(string))
	return plaintextDek, encryptedDek, nil
}

func (v *VaultTransitService) RotateKek(s session.Session, tenantId any) (int, error) {
	keyName := v.keyName(tenantId)
	exists, err := v.keyExists(s, tenantId)
	if err != nil {
		return 0, err
	}
	if !exists {
		_, err := v.request(s, http.MethodPost, "/keys/"+keyName, map[string]any{
			"type": v.keyType,
		})
		if err != nil {
			return 0, err
		}
		return 1, nil
	}
	if _, err := v.request(s, http.MethodPost, "/keys/"+keyName+"/rotate", map[string]any{}); err != nil {
		return 0, err
	}
	result, err := v.request(s, http.MethodGet, "/keys/"+keyName, nil)
	if err != nil {
		return 0, err
	}
	data := result["data"].(map[string]any)
	return int(data["latest_version"].(float64)), nil
}

func (v *VaultTransitService) RewrapDek(s session.Session, tenantId any, encryptedDek []byte) ([]byte, error) {
	result, err := v.request(s, http.MethodPost, "/rewrap/"+v.keyName(tenantId), map[string]any{
		"ciphertext": string(encryptedDek),
	})
	if err != nil {
		return nil, err
	}
	data := result["data"].(map[string]any)
	return []byte(data["ciphertext"].(string)), nil
}

func (v *VaultTransitService) DeleteKek(s session.Session, tenantId any) error {
	exists, err := v.keyExists(s, tenantId)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	keyName := v.keyName(tenantId)
	if _, err := v.request(s, http.MethodPost, "/keys/"+keyName+"/config", map[string]any{
		"deletion_allowed": true,
	}); err != nil {
		return err
	}
	_, err = v.request(s, http.MethodDelete, "/keys/"+keyName, nil)
	return err
}

func (v *VaultTransitService) Setup(s session.Session) error {
	return nil
}

func (v *VaultTransitService) Cleanup(s session.Session) error {
	return nil
}

func (v *VaultTransitService) keyName(tenantId any) string {
	return url.PathEscape(fmt.Sprint(tenantId))
}

func (v *VaultTransitService) keyExists(s session.Session, tenantId any) (bool, error) {
	_, err := v.request(s, http.MethodGet, "/keys/"+v.keyName(tenantId), nil)
	if err == ErrKekNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (v *VaultTransitService) ensureKey(s session.Session, tenantId any) error {
	_, err := v.request(s, http.MethodPost, "/keys/"+v.keyName(tenantId), map[string]any{
		"type": v.keyType,
	})
	return err
}

func (v *VaultTransitService) request(s session.Session, method, path string, data map[string]any) (map[string]any, error) {
	httpClient := extractHttpClient(s)
	reqUrl := fmt.Sprintf("%s/v1/%s%s", v.vaultAddr, v.mount, path)

	var body *bytes.Buffer
	if data != nil {
		body = &bytes.Buffer{}
		if err := json.NewEncoder(body).Encode(data); err != nil {
			return nil, err
		}
	}

	var reqBody *bytes.Reader
	if body != nil {
		reqBody = bytes.NewReader(body.Bytes())
	}

	var req *http.Request
	var err error
	if reqBody != nil {
		req, err = http.NewRequestWithContext(s.Context(), method, reqUrl, reqBody)
	} else {
		req, err = http.NewRequestWithContext(s.Context(), method, reqUrl, nil)
	}
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Vault-Token", v.vaultToken)
	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrKekNotFound
	}
	if resp.StatusCode == http.StatusNoContent {
		return map[string]any{}, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("vault: unexpected status %d for %s %s", resp.StatusCode, method, path)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}
