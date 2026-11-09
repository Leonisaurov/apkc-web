package main

// apkc rotate: crea el linaje de rotación a partir de dos keystores. El fichero que
// escribe es el mismo que entiende `apksigner --lineage`, así que el oráculo es firmar
// con él y ver que `apksigner` lo acepta. La clave nueva no se usa aquí: el linaje solo
// necesita su certificado; la firma de un nodo la pone la clave vieja.

import (
	"flag"
	"os"
)

func rotar() {
	set := flag.NewFlagSet("rotate", flag.ExitOnError)
	oldStore := set.String("old-keystore", keyStorePath, "keystore of the signing key being rotated from")
	oldPass := set.String("old-storepass", "android", "password of the old keystore")
	oldAlias := set.String("old-keyalias", "androiddebugkey", "alias in the old keystore")
	newStore := set.String("new-keystore", keyStorePath, "keystore of the signing key being rotated to")
	newPass := set.String("new-storepass", "android", "password of the new keystore")
	newAlias := set.String("new-keyalias", "androiddebugkey", "alias in the new keystore")
	out := set.String("out", "", "where to write the lineage file")
	set.Parse(os.Args[2:])

	if *out == "" {
		LogF("rotate", "falta --out: el linaje se escribe en un fichero",
			"(--out lineage.bin)")
	}
	oldSigner, oldCert, err := signingKey(*oldStore, *oldPass, *oldAlias)
	if err != nil {
		LogF("rotate", "old keystore:", err.Error())
	}
	_, newCert, err := signingKey(*newStore, *newPass, *newAlias)
	if err != nil {
		LogF("rotate", "new keystore:", err.Error())
	}
	if oldCert.Equal(newCert) {
		LogF("rotate", "los dos keystores tienen el mismo certificado:",
			"un linaje tiene que enlazar dos claves distintas")
	}
	oldKey, err := rsaKey(oldSigner)
	if err != nil {
		LogF("rotate", err.Error())
	}
	linaje, err := crearLinaje(oldCert, newCert, oldKey)
	if err != nil {
		LogF("rotate", err.Error())
	}
	if err := os.WriteFile(*out, linaje, 0644); err != nil {
		LogF("rotate", err.Error())
	}
	LogI("rotate", *out, "links", oldCert.Subject.CommonName, "->", newCert.Subject.CommonName)
}
