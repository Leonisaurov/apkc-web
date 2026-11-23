package main

import "testing"

// TestElTextoNoSeCongelaAlLlegarAlTopeDeCapturas cierra el defecto que congelaba el emulador en
// «Pulsos: 22»: cada clic del contador del studio genera tres capturas (TextView.setText, Log.d y
// println), así que la cota antigua de 64 se agotaba en el clic 23 mientras la consola seguía.
func TestElTextoNoSeCongelaAlLlegarAlTopeDeCapturas(t *testing.T) {
	apk := apkDelContador(t, fuentesDelContador(t)...)
	pulsos := make([]string, 32)
	for i := range pulsos {
		pulsos[i] = "id/pulsar"
	}
	modelo, err := modeloVistaDeAPKConPulsos(apk, "", true, pulsos)
	if err != nil {
		t.Fatalf("el preview con 32 clics falló: %v", err)
	}
	texto := modelo.Raiz.Hijos[0].TextoDinamico
	if texto != "Pulsos: 32" {
		t.Fatalf("tras 32 clics el modelo dice %q: se esperaba %q", texto, "Pulsos: 32")
	}
}
