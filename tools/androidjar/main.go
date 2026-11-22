package main

// Genera un android-stub.jar desde las definiciones publicas de AOSP. No abre
// el android.jar de la SDK: las clases se escriben desde current.txt y la tabla
// de recursos desde public-final.xml y attrs.xml.

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func main() {
	var (
		entrada = flag.String("in", "", "directorio con current.txt, public-final.xml, attrs.xml y attrs_manifest.xml")
		salida  = flag.String("out", "dist", "directorio de salida")
		apiNum  = flag.Int("api", 34, "nivel de API que se declara en PROCEDENCIA.md")
		tag     = flag.String("tag", "android-14.0.0_r75", "etiqueta de AOSP")
		only    = flag.String("only", "", "lista de clases separadas por comas; vacio = toda la API publica")
		notice  = flag.String("notice", "", "fichero NOTICE de AOSP que se copia al jar")
		license = flag.String("license", "", "texto de la licencia Apache-2.0 que se copia al jar")
	)
	flag.Parse()
	if *entrada == "" {
		fatal("-in es obligatorio")
	}
	if err := os.MkdirAll(*salida, 0o755); err != nil {
		fatal(err.Error())
	}

	recursos, err := leePublic(filepath.Join(*entrada, "public-final.xml"))
	if err != nil {
		fatal("public-final.xml: " + err.Error())
	}
	attrs := map[string]*defAtributo{}
	for _, n := range []string{"attrs.xml", "attrs_manifest.xml"} {
		if err := leeAttrs(filepath.Join(*entrada, n), attrs); err != nil {
			fatal(n + ": " + err.Error())
		}
	}
	recursos = anadeIdsDeItems(recursos, attrs)

	arsc, err := escribeARSC(recursos, attrs)
	if err != nil {
		fatal("resources.arsc: " + err.Error())
	}

	// libcore/api/current.txt va primero: java.*, javax.* y los paquetes de
	// org.* que el framework no declara. Asi, cuando se lee
	// android.widget.TextView, `CharSequence` ya se puede convertir a
	// `java/lang/CharSequence`. Los dos ficheros son Apache-2.0.
	apiCompleta := &api{clases: map[string]*apiClase{}}
	if lf, err := os.Open(filepath.Join(*entrada, "libcore-current.txt")); err == nil {
		if err := leeAPIEn(lf, apiCompleta); err != nil {
			_ = lf.Close()
			fatal("libcore-current.txt: " + err.Error())
		}
		_ = lf.Close()
	}
	f, err := os.Open(filepath.Join(*entrada, "current.txt"))
	if err != nil {
		fatal("current.txt: " + err.Error())
	}
	if err := leeAPIEn(f, apiCompleta); err != nil {
		_ = f.Close()
		fatal("current.txt: " + err.Error())
	}
	_ = f.Close()
	seleccion := map[string]bool{}
	if *only != "" {
		for _, n := range strings.Split(*only, ",") {
			n = strings.TrimSpace(n)
			if n != "" {
				seleccion[n] = true
			}
		}
	}
	clases := []*apiClase{}
	for _, n := range apiCompleta.orden {
		if len(seleccion) > 0 && !seleccion[n] {
			continue
		}
		clases = append(clases, apiCompleta.clases[n])
	}
	if len(clases) == 0 {
		fatal("no hay clases que generar")
	}
	sort.Slice(clases, func(i, j int) bool { return clases[i].interno < clases[j].interno })

	jar := filepath.Join(*salida, fmt.Sprintf("android-stub-%d.jar", *apiNum))
	if err := escribeJar(jar, clases, apiCompleta.clases, arsc, *notice, *license); err != nil {
		fatal("jar: " + err.Error())
	}
	prov := filepath.Join(*salida, "PROCEDENCIA.md")
	if err := escribeProcedencia(prov, *entrada, *tag, *apiNum, clases, len(recursos), len(attrs)); err != nil {
		fatal("PROCEDENCIA.md: " + err.Error())
	}
	fmt.Printf("android-stub-%d.jar: %d clases, %d recursos, %d atributos\n", *apiNum, len(clases), len(recursos), len(attrs))
}

func fatal(s string) {
	fmt.Fprintln(os.Stderr, "androidjar:", s)
	os.Exit(1)
}

type publicXML struct {
	Type string `xml:"type,attr"`
	Name string `xml:"name,attr"`
	ID   string `xml:"id,attr"`
}

func leePublic(ruta string) ([]recurso, error) {
	f, err := os.Open(ruta)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec := xml.NewDecoder(f)
	salida := []recurso{}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "public" {
			continue
		}
		var p publicXML
		if err := dec.DecodeElement(&p, &se); err != nil {
			return nil, err
		}
		if p.Type == "" || p.Name == "" {
			continue // los <public name="..."/> de los grupos staging no traen tipo
		}
		id := uint32(0)
		if p.ID != "" {
			v, err := strconv.ParseUint(p.ID, 0, 32)
			if err != nil {
				return nil, fmt.Errorf("%s/%s: id %q: %w", p.Type, p.Name, p.ID, err)
			}
			id = uint32(v)
			if id>>24 != 1 || int(byte(id>>16)) > len(tiposDelFramework) {
				continue // recurso de producto o de un tipo que no emitimos
			}
		}
		salida = append(salida, recurso{tipo: p.Type, nombre: p.Name, id: id})
	}
	// las entradas sin id toman el siguiente hueco de su tipo, en el orden del
	// fichero. Medido contra el jar de la SDK en el plan: 2.924/2.926.
	maxPorTipo := map[string]uint32{}
	for _, r := range salida {
		if r.id != 0 && r.id&0xffff > maxPorTipo[r.tipo]&0xffff {
			maxPorTipo[r.tipo] = r.id
		}
	}
	for i := range salida {
		if salida[i].id != 0 {
			continue
		}
		base := maxPorTipo[salida[i].tipo]
		if base == 0 {
			base = 0x01000000 | uint32(indiceTipo(salida[i].tipo)+1)<<16
		}
		base++
		for usado(salida, base) {
			base++
		}
		salida[i].id = base
		maxPorTipo[salida[i].tipo] = base
	}
	return salida, nil
}

func usado(r []recurso, id uint32) bool {
	for _, e := range r {
		if e.id == id {
			return true
		}
	}
	return false
}

func indiceTipo(t string) int {
	for i, n := range tiposDelFramework {
		if n == t {
			return i
		}
	}
	return -1
}

func anadeIdsDeItems(recursos []recurso, attrs map[string]*defAtributo) []recurso {
	tiene := map[string]bool{}
	max := uint32(0x01020000)
	for _, r := range recursos {
		if r.tipo == "id" {
			tiene[r.nombre] = true
			if r.id&0xffff > max&0xffff {
				max = r.id
			}
		}
	}
	nombres := []string{}
	for _, d := range attrs {
		for n := range d.items {
			if !tiene[n] {
				nombres = append(nombres, n)
			}
		}
	}
	sort.Strings(nombres)
	for _, n := range nombres {
		if tiene[n] {
			continue
		}
		max++
		for usado(recursos, max) {
			max++
		}
		recursos = append(recursos, recurso{tipo: "id", nombre: n, id: max})
		tiene[n] = true
	}
	return recursos
}

type itemXML struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

type attrXML struct {
	Name   string    `xml:"name,attr"`
	Format string    `xml:"format,attr"`
	Enums  []itemXML `xml:"enum"`
	Flags  []itemXML `xml:"flag"`
}

func leeAttrs(ruta string, defs map[string]*defAtributo) error {
	f, err := os.Open(ruta)
	if err != nil {
		return err
	}
	defer f.Close()
	dec := xml.NewDecoder(f)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "attr" {
			continue
		}
		var a attrXML
		if err := dec.DecodeElement(&a, &se); err != nil {
			return err
		}
		if a.Name == "" {
			continue
		}
		d := defs[a.Name]
		if d == nil {
			d = &defAtributo{items: map[string]int64{}}
			defs[a.Name] = d
		}
		if a.Format != "" {
			d.formato = formatoDe(a.Format)
			if strings.Contains(a.Format, "flags") {
				d.flags = true
			}
		}
		if len(a.Flags) > 0 {
			d.flags = true
			d.formato |= 0x00020000
		}
		for _, it := range append(a.Enums, a.Flags...) {
			v, err := strconv.ParseInt(strings.TrimSpace(it.Value), 0, 64)
			if err != nil {
				continue // valores simbolicos (?attr/...), no medidos
			}
			d.items[it.Name] = v
		}
	}
}

func escribeJar(ruta string, clases []*apiClase, todas map[string]*apiClase, arsc []byte, notice, license string) error {
	f, err := os.Create(ruta)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	for _, c := range clases {
		b, err := claseDeStub(c, todas)
		if err != nil {
			_ = zw.Close()
			_ = f.Close()
			return fmt.Errorf("%s: %w", c.nombre, err)
		}
		w, err := creaEntrada(zw, c.interno+".class")
		if err != nil {
			return err
		}
		if _, err := w.Write(b); err != nil {
			return err
		}
	}
	w, err := creaEntrada(zw, "resources.arsc")
	if err != nil {
		return err
	}
	if _, err := w.Write(arsc); err != nil {
		return err
	}
	for _, par := range []struct{ ruta, destino string }{{notice, "NOTICE"}, {license, "LICENSE"}} {
		if par.ruta == "" {
			continue
		}
		b, err := os.ReadFile(par.ruta)
		if err != nil {
			return err
		}
		w, err := creaEntrada(zw, par.destino)
		if err != nil {
			return err
		}
		if _, err := w.Write(b); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return f.Close()
}

// creaEntrada fija la fecha para que dos ejecuciones del mismo árbol den el
// mismo jar: `zip.Create` pone time.Now y el hash del artifact cambiaria en
// cada run.
func creaEntrada(zw *zip.Writer, nombre string) (io.Writer, error) {
	h := &zip.FileHeader{Name: nombre, Method: zip.Deflate}
	h.Modified = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
	return zw.CreateHeader(h)
}

func escribeProcedencia(ruta, entrada, tag string, api int, clases []*apiClase, recursos, attrs int) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# Procedencia del android-stub-%d.jar\n\n", api)
	fmt.Fprintf(&b, "Generado por `tools/androidjar` desde fuentes Apache-2.0 de AOSP.\n\n")
	fmt.Fprintf(&b, "- etiqueta: `%s`\n", tag)
	fmt.Fprintf(&b, "- API: %d\n", api)
	fmt.Fprintf(&b, "- clases: %d\n", len(clases))
	fmt.Fprintf(&b, "- recursos publicos: %d\n", recursos)
	fmt.Fprintf(&b, "- atributos: %d\n\n", attrs)
	fmt.Fprintf(&b, "## SHA-256 de las entradas\n\n")
	for _, n := range []string{"current.txt", "libcore-current.txt", "public-final.xml", "attrs.xml", "attrs_manifest.xml"} {
		bs, err := os.ReadFile(filepath.Join(entrada, n))
		if err != nil {
			continue
		}
		h := sha256.Sum256(bs)
		fmt.Fprintf(&b, "- `%s`: `%s`\n", n, hex.EncodeToString(h[:]))
	}
	fmt.Fprintf(&b, "\nNo contiene ninguna entrada copiada del `android.jar` de la SDK.\n")
	return os.WriteFile(ruta, []byte(b.String()), 0o644)
}
