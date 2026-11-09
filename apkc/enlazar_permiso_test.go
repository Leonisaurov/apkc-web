package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// fuentePermiso: un `<permission>` con label, description, icon y protectionLevel además de name.
// aapt2 escribe `protectionLevel` como entero hexadecimal (0x11) porque el atributo es de formato
// `flags`; copiar el tipo que guarda el framework (0x10) daba un AXML distinto (§138).
const fuentePermiso = `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.perm">
  <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="33"/>
  <permission android:name="com.perm.USAR" android:label="@string/app_name" android:description="@string/desc" android:icon="@drawable/i" android:protectionLevel="dangerous"/>
  <application android:label="@string/app_name" android:icon="@drawable/i"/>
</manifest>`

// TestPermisoConSusAtributos: el AXML del manifiesto sale byte a byte el de aapt2.
func TestPermisoConSusAtributos(t *testing.T) {
	valores := []fuenteXML{{nombre: "res/values/strings.xml", contenido: []byte(
		`<resources><string name="app_name">Perm</string><string name="desc">Una descripcion</string></resources>`)}}
	layout := []fuenteXML{{nombre: "res/layout/main.xml", contenido: []byte(
		`<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" android:layout_width="match_parent" android:layout_height="match_parent"/>`)}}
	png, err := base64.StdEncoding.DecodeString(pngPermiso)
	if err != nil {
		t.Fatal(err)
	}
	ficheros := []fuenteXML{{nombre: "res/drawable/i.png", contenido: png}}
	manifiesto := fuenteXML{nombre: "AndroidManifest.xml", contenido: []byte(fuentePermiso)}
	m, err := marcoDelAndroidJar(jarDelSDK(t))
	if err != nil {
		t.Fatalf("el android.jar no se lee: %v", err)
	}
	salida, err := enlazarProyecto(valores, layout, ficheros, manifiesto, m,
		opcionesDeEnlace{compileSdkVersion: 34, compileSdkCodename: "14"})
	if err != nil {
		t.Fatalf("el enlace no salio: %v", err)
	}
	nuestro, err := escribeAXML(salida.manifiesto)
	if err != nil {
		t.Fatalf("el manifiesto no se escribio: %v", err)
	}
	fijado, err := base64.StdEncoding.DecodeString(axmlPermiso)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(nuestro, fijado) {
		t.Errorf("el AXML nuestro mide %d bytes y el de aapt2 %d; el primero que difiere es el %d",
			len(nuestro), len(fijado), primerByteDistinto(nuestro, fijado))
	}
}

// axmlPermiso: `aapt2 link --no-auto-version` del manifiesto del test y `unzip -p app.apk AndroidManifest.xml | base64 -w0`.
const axmlPermiso = "AwAIAFQFAAABABwA6AIAABUAAAAAAAAAAAAAAHAAAAAAAAAAAAAAAA4AAAAaAAAAJgAAAEgAAABiAAAAgAAAAKQAAADKAAAAAAEAAAgBAAAaAQAANAEAAEgBAABmAQAAvgEAANIBAADkAQAA/AEAADACAABkAgAABQBsAGEAYgBlAGwAAAAEAGkAYwBvAG4AAAAEAG4AYQBtAGUAAAAPAHAAcgBvAHQAZQBjAHQAaQBvAG4ATABlAHYAZQBsAAAACwBkAGUAcwBjAHIAaQBwAHQAaQBvAG4AAAANAG0AaQBuAFMAZABrAFYAZQByAHMAaQBvAG4AAAAQAHQAYQByAGcAZQB0AFMAZABrAFYAZQByAHMAaQBvAG4AAAARAGMAbwBtAHAAaQBsAGUAUwBkAGsAVgBlAHIAcwBpAG8AbgAAABkAYwBvAG0AcABpAGwAZQBTAGQAawBWAGUAcgBzAGkAbwBuAEMAbwBkAGUAbgBhAG0AZQAAAAIAMQA0AAAABwBhAG4AZAByAG8AaQBkAAAACwBhAHAAcABsAGkAYwBhAHQAaQBvAG4AAAAIAGMAbwBtAC4AcABlAHIAbQAAAA0AYwBvAG0ALgBwAGUAcgBtAC4AVQBTAEEAUgAAACoAaAB0AHQAcAA6AC8ALwBzAGMAaABlAG0AYQBzAC4AYQBuAGQAcgBvAGkAZAAuAGMAbwBtAC8AYQBwAGsALwByAGUAcwAvAGEAbgBkAHIAbwBpAGQAAAAIAG0AYQBuAGkAZgBlAHMAdAAAAAcAcABhAGMAawBhAGcAZQAAAAoAcABlAHIAbQBpAHMAcwBpAG8AbgAAABgAcABsAGEAdABmAG8AcgBtAEIAdQBpAGwAZABWAGUAcgBzAGkAbwBuAEMAbwBkAGUAAAAYAHAAbABhAHQAZgBvAHIAbQBCAHUAaQBsAGQAVgBlAHIAcwBpAG8AbgBOAGEAbQBlAAAACAB1AHMAZQBzAC0AcwBkAGsAAACAAQgALAAAAAEAAQECAAEBAwABAQkAAQEgAAEBDAIBAXACAQFyBQEBcwUBAQABEAAYAAAAAQAAAP////8KAAAADgAAAAIBEACIAAAAAQAAAP//////////DwAAABQAFAAFAAAAAAAAAA4AAAAHAAAA/////wgAABAiAAAADgAAAAgAAAAJAAAACAAAAwkAAAD/////EAAAAAwAAAAIAAADDAAAAP////8SAAAA/////wgAABAiAAAA/////xMAAAD/////CAAAEA4AAAACARAATAAAAAIAAAD//////////xQAAAAUABQAAgAAAAAAAAAOAAAABQAAAP////8IAAAQFQAAAA4AAAAGAAAA/////wgAABAhAAAAAwEQABgAAAACAAAA//////////8UAAAAAgEQAIgAAAADAAAA//////////8RAAAAFAAUAAUAAAAAAAAADgAAAAAAAAD/////CAAAAQAAA38OAAAAAQAAAP////8IAAABAAABfw4AAAACAAAADQAAAAgAAAMNAAAADgAAAAMAAAD/////CAAAEQEAAAAOAAAABAAAAP////8IAAABAQADfwMBEAAYAAAAAwAAAP//////////EQAAAAIBEABMAAAABAAAAP//////////CwAAABQAFAACAAAAAAAAAA4AAAAAAAAA/////wgAAAEAAAN/DgAAAAEAAAD/////CAAAAQAAAX8DARAAGAAAAAQAAAD//////////wsAAAADARAAGAAAAAEAAAD//////////w8AAAABARAAGAAAAAEAAAD/////CgAAAA4AAAA="

// pngPermiso: el PNG de 1x1 del proyecto.
const pngPermiso = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="
