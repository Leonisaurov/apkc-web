package main

// El ensamblado del apk sin pasar por aapt2 (paso 3-E). Reune los artefactos que ya escriben
// arscwriter.go, axmlwriter.go y el dexer de la Fase 2, los mete en un zip con el mismo escritor que
// usa `apkc build` con el empaquetador de Go, y lo firma con el firmante v1+v2 que ya firma
// `apkc sign`. No hay una segunda vuelta por zipalign ni por apksigner: la alineacion se decide
// entrada a entrada y la firma se calcula sobre lo que va saliendo.
//
// Las dos reglas de formato que se copian del empaquetador de apkc, medidas alli: un entrada
// almacenada (sin comprimir) se alinea a cuatro bytes, y un recurso nativo a pagina; `resources.arsc`
// es la unica entrada de este apk que viaja almacenada, porque el sistema la mapea en memoria.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// entradaDePaquete es un fichero del apk: su nombre dentro y su contenido. sinComprimir guarda la
// forma almacenada y alineada que exige resources.arsc.
type entradaDePaquete struct {
	nombre       string
	contenido    []byte
	sinComprimir bool
}

// empaquetaSinAapt2 escribe en ruta el apk hecho con las entradas dadas, firmado con la clave pedida.
// El plan de entradas se materializa en ficheros temporales porque ese es el formato que consume el
// empaquetador de apkc: el mismo codigo que recorre el arbol del proyecto recorre aqui el paquete.
func empaquetaSinAapt2(ruta string, entradas []entradaDePaquete, keyPath, storePass, keyAlias string) error {
	if len(entradas) == 0 {
		return fmt.Errorf("el paquete no tiene entradas")
	}
	dir, err := os.MkdirTemp("", "apkc-paquete")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	plan := make([]bundleEntry, 0, len(entradas))
	for i, e := range entradas {
		// el nombre del fichero dentro del apk viene del proyecto, que es entrada de fuera: un
		// ".." o una barra inicial saldrian del paquete
		if e.nombre == "" || strings.HasPrefix(e.nombre, "/") || strings.Contains(e.nombre, "..") {
			return fmt.Errorf("entrada #%d con nombre %q: un fichero del apk no puede empezar con barra ni llevar ..", i, e.nombre)
		}
		src := filepath.Join(dir, fmt.Sprintf("entrada-%d", i))
		if err := os.WriteFile(src, e.contenido, 0o600); err != nil {
			return err
		}
		plan = append(plan, bundleEntry{src: src, name: e.nombre, compress: !e.sinComprimir})
	}

	tmp := ruta + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	// ninguna entrada se puede copiar de un paquete anterior: esto es un enlace desde cero
	reuse := make([]bool, len(plan))
	records, _, digests, err := writePackageEntries(out, plan, reuse, nil, map[string]string{}, true)
	if err != nil {
		out.Close()
		return err
	}
	signer, cert, err := signingKey(keyPath, storePass, keyAlias)
	if err != nil {
		out.Close()
		return err
	}
	f, err := firmanteDe(signer, cert)
	if err != nil {
		out.Close()
		return err
	}
	return finishSignedPackage(out, tmp, ruta, records, plan, digests, f)
}
