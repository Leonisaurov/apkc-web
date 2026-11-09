package main

// apkc keystore: what the pestaña needs to show before it signs. A file picker gives it
// bytes; this command answers which entries the file holds, the name of each one, the
// subject and SHA-256 of its certificate, and until when that certificate is valid. With
// that the page can show the identity, require an alias when the file has several, and
// never sign with a key it did not name.
//
// The output is one line of JSON with a fixed prefix, so the page can parse it without
// scraping the log lines. Nothing here starts a process: the same PKCS#12 and JKS readers
// the signer uses, so it runs inside wasm.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

type keystoreEntryInfo struct {
	Alias    string `json:"alias"`
	Subject  string `json:"subject"`
	SHA256   string `json:"sha256"`
	NotAfter string `json:"notAfter"`
}

type keystoreInfo struct {
	Keys    int                 `json:"keys"`
	Entries []keystoreEntryInfo `json:"entries"`
}

// inspectarKeystore opens the file with the reader its header names and lists every
// signing identity. A keystore that cannot be opened at all is an error; a key whose
// certificate is missing is listed without one rather than hidden, because that is
// exactly the case the page should refuse before signing.
func inspectarKeystore(path, password string) (keystoreInfo, error) {
	blob, err := os.ReadFile(path)
	if err != nil {
		return keystoreInfo{}, err
	}
	reader := readPKCS12
	if isJKSHead(blob) {
		reader = readJKS
	}
	material, err := reader(blob, password, "")
	if err != nil && len(material.Entries) == 0 {
		return keystoreInfo{}, fmt.Errorf("%s: %v", path, err)
	}
	info := keystoreInfo{Keys: material.Keys, Entries: make([]keystoreEntryInfo, 0, len(material.Entries))}
	for _, e := range material.Entries {
		entrada := keystoreEntryInfo{Alias: e.alias}
		if e.cert != nil {
			entrada.Subject = e.cert.Subject.String()
			entrada.SHA256 = huellaSHA256(e.cert.Raw)
			entrada.NotAfter = e.cert.NotAfter.Format("2006-01-02")
		}
		info.Entries = append(info.Entries, entrada)
	}
	return info, nil
}

// huellaSHA256 is the fingerprint a person compares between two tools: uppercase hex,
// one colon between bytes.
func huellaSHA256(der []byte) string {
	sum := sha256.Sum256(der)
	hexa := strings.ToUpper(hex.EncodeToString(sum[:]))
	var b strings.Builder
	for i := 0; i < len(hexa); i += 2 {
		if i > 0 {
			b.WriteByte(':')
		}
		b.WriteString(hexa[i : i+2])
	}
	return b.String()
}

func keystore() {
	set := flag.NewFlagSet("keystore", flag.ExitOnError)
	keyStore := set.String("keystore", keyStorePath, "path to the keystore")
	storePass := set.String("storepass", "android", "keystore password")
	set.Parse(os.Args[2:])

	info, err := inspectarKeystore(*keyStore, *storePass)
	if err != nil {
		LogF("keystore", err.Error())
	}
	datos, err := json.Marshal(info)
	if err != nil {
		LogF("keystore", err.Error())
	}
	// una sola linea, con prefijo: la pagina la busca y el resto es la consola normal
	fmt.Println("keystore-json: " + string(datos))
}
