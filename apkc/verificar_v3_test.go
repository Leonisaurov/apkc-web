package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// paqueteConV3 firma el fixture con v1+v2 y después cambia el bloque por uno con v2+v3 calculado
// sobre el contenido real. El verificador vuelve a leer el fichero desde el disco: no comparte
// estado con quien lo firma.
func paqueteConV3(t *testing.T) string {
	t.Helper()
	key, cert := signingPair(t)
	firmado := filepath.Join(t.TempDir(), "v3.apk")
	if _, err := resignWithKey(fixturePackage(t), firmado, key, cert); err != nil {
		t.Fatalf("no se pudo firmar el fixture: %v", err)
	}
	f, err := os.Open(firmado)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	eocd, err := leerRegistroFinal(f, info.Size())
	if err != nil {
		t.Fatal(err)
	}
	bloque, err := leerBloqueDeFirmas(f, eocd)
	if err != nil || bloque == nil {
		t.Fatalf("el paquete no traía bloque de firmas: %v", err)
	}
	signer, err := firmanteDe(key, cert)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := digestDelContenido(f, eocd, bloque, info.Size(), signer.contentHash)
	if err != nil {
		t.Fatal(err)
	}
	v2, err := v2SignerBlock(digest, signer)
	if err != nil {
		t.Fatal(err)
	}
	v3, err := v3SignerBlock(digest, signer)
	if err != nil {
		t.Fatal(err)
	}
	todo, err := os.ReadFile(firmado)
	if err != nil {
		t.Fatal(err)
	}
	nuevo := apkSigningBlock(v2, v3)
	salida := append([]byte{}, todo[:bloque.inicio]...)
	offset := uint32(len(salida) + len(nuevo))
	salida = append(salida, nuevo...)
	cola := append([]byte{}, todo[bloque.inicio+bloque.largo:]...)
	if len(cola) < tamanoRegistroFinal {
		t.Fatal("la cola no contiene el registro final")
	}
	binary.LittleEndian.PutUint32(cola[len(cola)-tamanoRegistroFinal+sitioDesplazamiento:], offset)
	salida = append(salida, cola...)
	resultado := filepath.Join(t.TempDir(), "con-v3.apk")
	if err := os.WriteFile(resultado, salida, 0644); err != nil {
		t.Fatal(err)
	}
	return resultado
}

func TestVerificarV3ConFirmaVerdadera(t *testing.T) {
	v, err := verificarApk(paqueteConV3(t))
	if err != nil {
		t.Fatalf("el verificador no abrió el paquete: %v", err)
	}
	if !v.v1 || !v.v2 || !v.v3 {
		t.Errorf("v1=%v v2=%v v3=%v; motivos v3: %s", v.v1, v.v2, v.v3, v.v3Motivos)
	}
	if v.firmantesV3 != 1 {
		t.Errorf("firmantes v3 = %d", v.firmantesV3)
	}
}

func TestVerificarV3RechazaContenidoAlterado(t *testing.T) {
	ruta := paqueteConV3(t)
	b, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	// un byte de classes.dex cambia el digest de los tres tramos; la firma v3 sobre signedData
	// sigue siendo válida, pero el digest guardado ya no es el del contenido.
	i := indiceDeBytes(b, []byte("classes.dex"))
	if i < 0 {
		t.Fatal("el fixture no trae classes.dex")
	}
	b[i+5] ^= 1
	alterado := filepath.Join(t.TempDir(), "alterado.apk")
	if err := os.WriteFile(alterado, b, 0644); err != nil {
		t.Fatal(err)
	}
	v, err := verificarApk(alterado)
	if err != nil {
		t.Fatal(err)
	}
	if v.v3 || len(v.v3Motivos) == 0 {
		t.Errorf("la v3 no debía aprobar con el contenido alterado: motivos %s", v.v3Motivos)
	}
}

func indiceDeBytes(b, patron []byte) int {
	for i := 0; i+len(patron) <= len(b); i++ {
		if string(b[i:i+len(patron)]) == string(patron) {
			return i
		}
	}
	return -1
}
