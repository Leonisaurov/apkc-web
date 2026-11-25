package main

// La fusión del manifiesto de las dependencias. Solo se juntan los hijos directos de
// `<manifest>` y de `<application>`: los atributos del proyecto no se tocan y `R.txt` no
// entra aquí, porque `R.java` se regenera del enlace. La prioridad es proyecto > isla de
// `res/__deps/` > `.aar` de `jar/`, la misma que la de los `values`, y no depende del
// orden en que se lean los artefactos. Lo que use `tools:` se rechaza con nombre: este
// fork no modela esas directivas y no puede fingir que las aplicó.

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const espacioToolsManifiesto = "http://schemas.android.com/tools"
const espacioAndroidManifiesto = "http://schemas.android.com/apk/res/android"

// hijoManifiesto es un hijo directo de `<manifest>` o de `<application>` tal como viene
// en el fichero: el texto exacto del elemento se guarda para insertarlo sin reescribirlo.
type hijoManifiesto struct {
	etiqueta            string
	nombre              string
	crudo               []byte
	dentroDeApplication bool
}

func (h hijoManifiesto) clave() string {
	if h.dentroDeApplication {
		return "a\x00" + h.etiqueta + "\x00" + h.nombre
	}
	return "m\x00" + h.etiqueta + "\x00" + h.nombre
}

type fuenteManifiesto struct {
	contenido []byte
	origen    string
	prioridad int
}

type candidatoManifiesto struct {
	clave     string
	hijo      hijoManifiesto
	origen    string
	prioridad int
}

// manifiestoConDependencias devuelve el manifiesto del proyecto con los hijos que aportan
// las islas y los AAR que aún no estén en él. Un proyecto sin dependencias sale igual.
func manifiestoConDependencias(raiz string, base fuenteXML) (fuenteXML, error) {
	fuentes := []fuenteManifiesto{{contenido: base.contenido, origen: "el proyecto", prioridad: 0}}

	islas, err := os.ReadDir(filepath.Join(raiz, "res", carpetaDeps))
	if err != nil && !os.IsNotExist(err) {
		return base, err
	}
	for _, isla := range islas {
		if !isla.IsDir() {
			continue
		}
		datos, err := os.ReadFile(filepath.Join(raiz, "res", carpetaDeps, isla.Name(), "AndroidManifest.xml"))
		if err != nil {
			continue
		}
		fuentes = append(fuentes, fuenteManifiesto{datos, "la isla " + isla.Name(), 1})
	}

	aares, err := aarsExtraidos(raiz)
	if err != nil {
		return base, err
	}
	for _, aar := range aares {
		if aar.manifiesto == "" {
			continue
		}
		datos, err := os.ReadFile(aar.manifiesto)
		if err != nil {
			return base, fmt.Errorf("el manifiesto de %s no se puede leer: %w", filepath.Base(aar.aar), err)
		}
		fuentes = append(fuentes, fuenteManifiesto{datos, "el AAR " + filepath.Base(aar.aar), 2})
	}

	ganadores := map[string]candidatoManifiesto{}
	for _, fuente := range fuentes {
		hijos, err := hijosDelManifiesto(fuente.contenido, fuente.origen)
		if err != nil {
			return base, err
		}
		for _, hijo := range hijos {
			clave := hijo.clave()
			anterior, existe := ganadores[clave]
			if existe {
				LogW("build", "el elemento", hijo.etiqueta, hijo.nombre, "de", fuente.origen,
					"ya lo aporta", anterior.origen, ": gana el de mayor prioridad")
				if fuente.prioridad < anterior.prioridad {
					ganadores[clave] = candidatoManifiesto{clave, hijo, fuente.origen, fuente.prioridad}
				}
				continue
			}
			ganadores[clave] = candidatoManifiesto{clave, hijo, fuente.origen, fuente.prioridad}
		}
	}

	delManifest := make([][]byte, 0)
	deLaAplicacion := make([][]byte, 0)
	orden := make([]candidatoManifiesto, 0, len(ganadores))
	for _, candidato := range ganadores {
		orden = append(orden, candidato)
	}
	sort.Slice(orden, func(i, j int) bool { return orden[i].clave < orden[j].clave })
	for _, candidato := range orden {
		if candidato.origen == "el proyecto" {
			continue
		}
		if candidato.hijo.dentroDeApplication {
			deLaAplicacion = append(deLaAplicacion, candidato.hijo.crudo)
		} else {
			delManifest = append(delManifest, candidato.hijo.crudo)
		}
	}

	salida := append([]byte(nil), base.contenido...)
	if len(deLaAplicacion) > 0 {
		salida, err = insertaEnAplicacion(salida, deLaAplicacion)
		if err != nil {
			return base, err
		}
	}
	if len(delManifest) > 0 {
		salida, err = insertaAntesDelCierre(salida, "</manifest>", delManifest)
		if err != nil {
			return base, err
		}
	}
	base.contenido = salida
	return base, nil
}

// hijosDelManifiesto lee los hijos directos de los dos contenedores que se fusionan y
// guarda su texto exacto. Una directiva `tools:` se rechaza antes de escribir nada.
func hijosDelManifiesto(datos []byte, origen string) ([]hijoManifiesto, error) {
	if bytes.Contains(datos, []byte("tools:")) {
		return nil, fmt.Errorf("el manifiesto de %s usa tools:, que este fork no modela", origen)
	}
	d := xml.NewDecoder(bytes.NewReader(datos))
	salida := []hijoManifiesto{}
	inicios := []int64{}
	padres := []string{}
	capturados := []int{}
	for {
		antes := d.InputOffset()
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("el manifiesto de %s no se lee: %w", origen, err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			for _, a := range t.Attr {
				if a.Name.Space == espacioToolsManifiesto || strings.HasPrefix(a.Name.Local, "tools:") {
					return nil, fmt.Errorf("el manifiesto de %s usa tools:%s en <%s>, que este fork no modela",
						origen, a.Name.Local, t.Name.Local)
				}
			}
			padre := ""
			if len(padres) > 0 {
				padre = padres[len(padres)-1]
			}
			profundidad := len(inicios)
			hijo := -1
			if (padre == "manifest" && profundidad == 1 && t.Name.Local != "application") ||
				(padre == "application" && profundidad == 2) {
				nombre := nombreAndroidDe(t.Attr)
				salida = append(salida, hijoManifiesto{
					etiqueta:            t.Name.Local,
					nombre:              nombre,
					dentroDeApplication: padre == "application",
				})
				hijo = len(salida) - 1
			}
			inicios = append(inicios, antes)
			padres = append(padres, t.Name.Local)
			capturados = append(capturados, hijo)
		case xml.EndElement:
			if len(inicios) == 0 {
				continue
			}
			inicio := inicios[len(inicios)-1]
			indice := capturados[len(capturados)-1]
			inicios = inicios[:len(inicios)-1]
			padres = padres[:len(padres)-1]
			capturados = capturados[:len(capturados)-1]
			if indice >= 0 && inicio >= 0 && int(inicio) <= len(datos) {
				salida[indice].crudo = datos[inicio:int(d.InputOffset())]
			}
		}
	}
	return salida, nil
}

func nombreAndroidDe(attrs []xml.Attr) string {
	for _, a := range attrs {
		if a.Name.Local == "name" && (a.Name.Space == espacioAndroidManifiesto || a.Name.Space == "") {
			return a.Value
		}
	}
	return ""
}

// insertaAntesDelCierre mete los bloques delante de la etiqueta de cierre que se pida.
func insertaAntesDelCierre(datos []byte, cierre string, bloques [][]byte) ([]byte, error) {
	i := bytes.LastIndex(datos, []byte(cierre))
	if i < 0 {
		return nil, fmt.Errorf("el manifiesto del proyecto no tiene %s", cierre)
	}
	bloque := append([]byte("\n    "), bytes.Join(bloques, []byte("\n    "))...)
	bloque = append(bloque, '\n')
	salida := make([]byte, 0, len(datos)+len(bloque))
	salida = append(salida, datos[:i]...)
	salida = append(salida, bloque...)
	salida = append(salida, datos[i:]...)
	return salida, nil
}

// insertaEnAplicacion mete los hijos dentro de `<application>`. Si el proyecto no tiene
// ese elemento lo crea, y si era autocerrado lo abre y lo cierra alrededor de los hijos.
func insertaEnAplicacion(datos []byte, hijos [][]byte) ([]byte, error) {
	i := bytes.Index(datos, []byte("<application"))
	if i < 0 {
		return insertaAntesDelCierre(datos, "</manifest>", [][]byte{
			append(append([]byte("<application>"), bytes.Join(hijos, []byte("\n        "))...), []byte("</application>")...),
		})
	}
	fin := finDeEtiqueta(datos, i)
	if fin < 0 || fin > len(datos) {
		return nil, fmt.Errorf("el manifiesto del proyecto tiene <application> sin cerrar")
	}
	cuerpo := bytes.Join(hijos, []byte("\n        "))
	if datos[fin-1] == '/' {
		apertura := append([]byte(nil), datos[i:fin-1]...)
		apertura = append(apertura, '>')
		bloque := append([]byte("\n        "), cuerpo...)
		bloque = append(bloque, []byte("\n    </application>")...)
		salida := make([]byte, 0, len(datos)+len(bloque)+1)
		salida = append(salida, datos[:i]...)
		salida = append(salida, apertura...)
		salida = append(salida, bloque...)
		salida = append(salida, datos[fin:]...)
		return salida, nil
	}
	j := bytes.Index(datos[fin:], []byte("</application>"))
	if j < 0 {
		return nil, fmt.Errorf("el manifiesto del proyecto tiene <application> sin cierre")
	}
	cierre := fin + j
	bloque := append([]byte("\n        "), cuerpo...)
	bloque = append(bloque, []byte("\n    ")...)
	salida := make([]byte, 0, len(datos)+len(bloque))
	salida = append(salida, datos[:cierre]...)
	salida = append(salida, bloque...)
	salida = append(salida, datos[cierre:]...)
	return salida, nil
}

// finDeEtiqueta devuelve el índice justo después del '>' de una etiqueta, sin confundir
// los '>' que pueda haber dentro de un valor entre comillas.
func finDeEtiqueta(datos []byte, inicio int) int {
	var comilla byte
	for i := inicio; i < len(datos); i++ {
		c := datos[i]
		if comilla != 0 {
			if c == comilla {
				comilla = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			comilla = c
			continue
		}
		if c == '>' {
			return i + 1
		}
	}
	return -1
}
