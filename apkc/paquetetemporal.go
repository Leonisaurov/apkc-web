package main

// El paquete y la firma se escriben primero en un temporal del mismo directorio y luego se
// renombran. Ese temporal no puede llevar un nombre derivado del destino: `ruta + ".tmp"` es
// predecible y, en un directorio compartido, deja que otro usuario plante allí un enlace simbólico
// antes de la escritura, con lo que los bytes del paquete acaban donde apunte el enlace. CreateTemp
// abre con O_EXCL y un nombre aleatorio, así que ni sigue un enlace que ya exista ni reutiliza uno.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// abrirTemporalDePaquete abre un temporal junto a ruta, en el mismo sistema de ficheros, para que
// el rename final sea atómico. Devuelve el fichero y su ruta; quien lo llama borra la ruta si no
// llega a renombrarla.
func abrirTemporalDePaquete(ruta string) (*os.File, string, error) {
	f, err := os.CreateTemp(filepath.Dir(ruta), ".apkc-*")
	if err != nil {
		return nil, "", err
	}
	return f, f.Name(), nil
}

// resolverPassword acepta la contraseña literal o la forma "file:<ruta>" que usa apksigner. La
// forma con fichero es la que evita que la contraseña viaje en una línea de órdenes, donde la lee
// cualquiera que liste procesos.
func resolverPassword(valor string) (string, error) {
	ruta, conFichero := strings.CutPrefix(valor, "file:")
	if !conFichero {
		return valor, nil
	}
	datos, err := os.ReadFile(ruta)
	if err != nil {
		return "", fmt.Errorf("no se puede leer la contraseña de %s: %w", ruta, err)
	}
	// Un fichero escrito con `echo` lleva un salto de línea final que no forma parte de la
	// contraseña; apksigner lo recorta igual al leer su propio file:.
	return strings.TrimRight(string(datos), "\r\n"), nil
}

// escribirPasswordTemporal deja la contraseña en un fichero de solo el dueño y devuelve su ruta,
// que el llamador borra. Es el puente entre una contraseña que llega en memoria y un programa
// externo que sabe leerla con --ks-pass file:.
func escribirPasswordTemporal(prefijo, password string) (string, error) {
	f, err := os.CreateTemp("", prefijo)
	if err != nil {
		return "", err
	}
	ruta := f.Name()
	if _, err := f.WriteString(password); err != nil {
		f.Close()
		os.Remove(ruta)
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(ruta)
		return "", err
	}
	return ruta, nil
}
