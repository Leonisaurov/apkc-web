package main

import (
	"encoding/xml"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type Manifest struct {
	XMLName     xml.Name    `xml:"manifest"`
	Package     string      `xml:"package,attr"`
	UsesSDK     UsesSDK     `xml:"uses-sdk"`
	Application Application `xml:"application"`
}

type UsesSDK struct {
	XMLName       xml.Name `xml:"uses-sdk"`
	MinSDKVersion string   `xml:"minSdkVersion,attr"`
}

// manifestMinAPI is the minSdkVersion of the project, or 0 when the manifest
// does not declare one. d8 needs it to decide how much desugaring to do, and
// without it it assumes API 1 and rewrites every lambda of every dependency.
func manifestMinAPI() int {
	mb, err := os.ReadFile("AndroidManifest.xml")
	if err != nil {
		return 0
	}

	manifest := Manifest{}
	if err := xml.Unmarshal(mb, &manifest); err != nil {
		return 0
	}

	api, err := strconv.Atoi(strings.TrimSpace(manifest.UsesSDK.MinSDKVersion))
	if err != nil || api < 1 {
		return 0
	}
	return api
}

// manifestChanged decides the manifest by content instead of by timestamp. The
// package name, the permissions and minSdkVersion all live in these bytes, and a
// copy that puts the same bytes back (a tool re-writing the merged manifest, a
// checkout, a cp -p restore) is not an edit: by timestamp it would throw away every
// compiled resource and every compiled source of the project for nothing. The caller
// stores the table only when this returns true, which is exactly when the digest
// below has to be kept.
func manifestChanged() bool {
	digest, err := fileSHA1("AndroidManifest.xml")
	if err != nil {
		return true
	}
	if digest == DataTable.ManifestDigest {
		return false
	}
	DataTable.ManifestDigest = digest
	return true
}

// manifestNativeLibsStored reports how lib/ has to go into the package, which is
// the manifest's decision and not apkc's: android:extractNativeLibs="false"
// promises the runtime that it can map a .so straight out of the apk, and that
// only holds when the entry is stored (not deflated) and page aligned. Anything
// else, including the attribute being absent, keeps the libraries deflated and
// lets the package manager unpack them at install.
func manifestNativeLibsStored() (stored bool, declared bool) {
	mb, err := os.ReadFile("AndroidManifest.xml")
	if err != nil {
		return false, false
	}

	manifest := Manifest{}
	if err := xml.Unmarshal(mb, &manifest); err != nil {
		return false, false
	}

	switch strings.TrimSpace(manifest.Application.ExtractNativeLibs) {
	case "":
		return false, false
	case "false":
		return true, true
	default:
		return false, true
	}
}

type Application struct {
	XMLName    xml.Name   `xml:"application"`
	Activities []Activity `xml:"activity"`
	// ExtractNativeLibs is the attribute the pairing above is read from. It is
	// kept as a string because the manifest also carries "@bool/reference" here,
	// and only the literal "false" asks for the stored layout.
	ExtractNativeLibs string `xml:"extractNativeLibs,attr"`
}

func (a Application) MainActivity() string {
	for _, a := range a.Activities {
		for _, i := range a.IntentFilter {
			for _, n := range i.Actions {
				if n.Name == "android.intent.action.MAIN" {
					return a.Name
				}
			}
		}
	}
	return ""
}

type Activity struct {
	XMLName      xml.Name       `xml:"activity"`
	Name         string         `xml:"name,attr"`
	IntentFilter []IntentFilter `xml:"intent-filter"`
}

type IntentFilter struct {
	XMLName xml.Name `xml:"intent-filter"`
	Actions []Action `xml:"action"`
}

type Action struct {
	XMLName xml.Name `xml:"action"`
	Name    string   `xml:"name,attr"`
}

// El manifiesto es un fichero que puede venir de fuera (un zip importado, un proyecto
// ajeno), y `apkc run` mete el paquete y la actividad dentro de un script que se ejecuta
// en la shell del dispositivo: `am start -W -S -n <paquete>/<actividad>` y `pidof -s
// <paquete>`. Un manifiesto con package="x; reboot" convertía ese script en dos órdenes.
// Se valida la forma antes de construir ninguna, y el patrón es el que Android acepta de
// todas formas: letras, dígitos y guion bajo, con al menos un punto.
var patronPaquete = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*(\.[A-Za-z0-9_]+)+$`)

// La actividad va en el mismo script y admite dos formas: absoluta (`com.x.MainActivity`)
// y relativa al paquete (`.MainActivity`). Es un nombre Java, así que además de letras y
// dígitos lleva `_` y `$` (clases internas); nada más pasa.
var patronActividad = regexp.MustCompile(`^\.?[A-Za-z_$][A-Za-z0-9_$]*(\.[A-Za-z_$][A-Za-z0-9_$]*)*$`)

// validarPaqueteYActividad juzga los dos valores del manifiesto que acaban dentro de una
// orden de shell. El mensaje nombra el valor y el motivo: quien importe el proyecto tiene
// que saber qué corregir, no solo que se rechazó.
func validarPaqueteYActividad(pkg, actividad string) error {
	if !patronPaquete.MatchString(pkg) {
		return fmt.Errorf("el paquete del manifiesto %q no es un nombre de paquete Android "+
			"(se esperaba algo como com.ejemplo.app, sin espacios, signos ni puntos seguidos)", pkg)
	}
	if !patronActividad.MatchString(actividad) {
		return fmt.Errorf("la actividad principal %q no es un nombre Java permitido "+
			"(se esperaba MainActivity, .MainActivity o com.ejemplo.app.MainActivity)", actividad)
	}
	return nil
}
