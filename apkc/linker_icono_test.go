package main

import (
	"bytes"
	"testing"
)

// TestElDrawableSeEnlazaComoArchivo: un PNG de res/drawable viaja tal cual al build y su entrada es
// una cadena con la ruta; el tipo drawable sale con id 1 (antes que layout). Medido con
// `aapt2 dump resources` en el parrafo 93.
func TestElDrawableSeEnlazaComoArchivo(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
	dir := proyecto(t, map[string]string{
		"AndroidManifest.xml":    manifestMinimo,
		"res/values/cadenas.xml": valuesMinimo,
		"res/layout/main.xml":    `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android"/>`,
		"res/drawable/icono.png": string(png),
	})
	valores, layouts, ficheros, manifiesto, err := fuentesDelProyecto(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(ficheros) != 1 || ficheros[0].nombre != "res/drawable/icono.png" {
		t.Fatalf("los ficheros salen %+v", ficheros)
	}
	if !bytes.Equal(ficheros[0].contenido, png) {
		t.Fatal("los bytes del PNG no llegan enteros a fuenteXML")
	}
	salida, err := enlazarProyecto(valores, layouts, ficheros, manifiesto, marcoDelEnlace(),
		opcionesDeEnlace{compileSdkVersion: 34, compileSdkCodename: "14"})
	if err != nil {
		t.Fatalf("el enlace no salio: %v", err)
	}
	if len(salida.tabla.tipos) < 1 || salida.tabla.tipos[0].id != 1 {
		t.Fatalf("los tipos salen %+v", salida.tabla.tipos)
	}
	if salida.tabla.nombresTipo[0] != "drawable" {
		t.Errorf("el primer tipo es %q, se esperaba drawable", salida.tabla.nombresTipo[0])
	}
	en := salida.tabla.tipos[0].bloques[0].entradas
	if len(en) != 1 || salida.tabla.claves[en[0].clave] != "icono" {
		t.Fatalf("las entradas de drawable salen %+v", en)
	}
	if en[0].valor.tipo != tipoCadena {
		t.Errorf("la entrada del drawable es tipo %#x, se esperaba cadena", en[0].valor.tipo)
	}
	if !bytes.Equal(salida.ficheros["res/drawable/icono.png"], png) {
		t.Error("el fichero del enlace no es el PNG de entrada")
	}
}
