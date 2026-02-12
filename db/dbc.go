package db

import (
	"bufio"
	"crypto/des"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// DBCConfig represents parsed DBC file configuration
type DBCConfig struct {
	GuestUserPwd     string
	AppsServletAgent string
	DBCName          string
	Host             string
	Port             string
	SID              string
	Domain           string
}

// ParseDBCFile reads and parses an EBS DBC file
// DBC files are in the format key=value and some values may be encrypted
func ParseDBCFile(dbcFilePath string) (*DBCConfig, error) {
	file, err := os.Open(dbcFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open DBC file: %w", err)
	}
	defer file.Close()

	config := &DBCConfig{}
	scanner := bufio.NewScanner(file)
	
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		// Parse key=value pairs
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		
		switch key {
		case "GUEST_USER_PWD":
			// GUEST_USER_PWD is encrypted, needs decryption
			config.GuestUserPwd, err = decryptDBCValue(value)
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt GUEST_USER_PWD: %w", err)
			}
		case "APPS_SERVLET_AGENT":
			config.AppsServletAgent = value
		case "DBC_NAME":
			config.DBCName = value
		case "HOST":
			config.Host = value
		case "PORT":
			config.Port = value
		case "SID":
			config.SID = value
		case "DOMAIN":
			config.Domain = value
		}
	}
	
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading DBC file: %w", err)
	}
	
	// Validate required fields
	if config.Host == "" {
		return nil, fmt.Errorf("HOST not found in DBC file")
	}
	if config.Port == "" {
		return nil, fmt.Errorf("PORT not found in DBC file")
	}
	if config.SID == "" {
		return nil, fmt.Errorf("SID not found in DBC file")
	}
	
	return config, nil
}

// decryptDBCValue decrypts a DBC encrypted value
// EBS uses DES encryption with a known key for DBC files
func decryptDBCValue(encryptedHex string) (string, error) {
	// Remove any whitespace
	encryptedHex = strings.ReplaceAll(encryptedHex, " ", "")
	
	// If the value doesn't look encrypted (no hex chars), return as-is
	if len(encryptedHex) < 16 || !isHexString(encryptedHex) {
		return encryptedHex, nil
	}
	
	// Decode hex string
	ciphertext, err := hex.DecodeString(encryptedHex)
	if err != nil {
		return "", fmt.Errorf("failed to decode hex: %w", err)
	}
	
	// EBS DBC encryption key (this is a well-known key used by EBS)
	// The actual key varies by EBS version, this is a simplified version
	// In production, this should be configurable or derived properly
	key := []byte("05101999") // 8 bytes for DES
	
	// Create DES cipher
	block, err := des.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}
	
	// Decrypt in ECB mode (EBS uses ECB mode for DBC)
	plaintext := make([]byte, len(ciphertext))
	
	// Process in 8-byte blocks
	for i := 0; i < len(ciphertext); i += 8 {
		block.Decrypt(plaintext[i:i+8], ciphertext[i:i+8])
	}
	
	// Remove padding and convert to string
	result := strings.TrimRight(string(plaintext), "\x00")
	
	return result, nil
}

// isHexString checks if a string contains only hex characters
func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// BuildConnectionString creates an Oracle connection string from DBC config
func (d *DBCConfig) BuildConnectionString() string {
	// Format: host:port/sid
	return fmt.Sprintf("%s:%s/%s", d.Host, d.Port, d.SID)
}

// GetTNSDescriptor creates a full TNS descriptor from DBC config
func (d *DBCConfig) GetTNSDescriptor() string {
	// Build a full TNS descriptor
	return fmt.Sprintf(
		"(DESCRIPTION=(ADDRESS=(PROTOCOL=TCP)(HOST=%s)(PORT=%s))(CONNECT_DATA=(SID=%s)))",
		d.Host, d.Port, d.SID,
	)
}
