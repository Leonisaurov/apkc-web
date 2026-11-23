package main

// El presupuesto de temporales: un bloque `/range` que necesita más registros que max_stack tiene
// que hacer subir la base del reparto, no reventar. Lo destapó `GravityCompat$...apply` con un
// `invokestatic` de más palabras que la pila medida.

import (
	"encoding/base64"
	"testing"
)

// gravityJavac: `GravityCompat$GravityCompatImplJellybeanMr1.class` del android-support-v4.jar,
// `base64 -w0`.
const gravityJavac = "yv66vgAAADEANAoABwAnCgAoACkKACgAKgoAKAArCgAoACwHAC4HAC8HADABAAY8aW5pdD4BAAMoKVYBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQASTG9jYWxWYXJpYWJsZVRhYmxlAQAEdGhpcwEAHUdyYXZpdHlDb21wYXRJbXBsSmVsbHliZWFuTXIxAQAMSW5uZXJDbGFzc2VzAQBFTGFuZHJvaWQvc3VwcG9ydC92NC92aWV3L0dyYXZpdHlDb21wYXQkR3Jhdml0eUNvbXBhdEltcGxKZWxseWJlYW5NcjE7AQASZ2V0QWJzb2x1dGVHcmF2aXR5AQAFKElJKUkBAAdncmF2aXR5AQABSQEAD2xheW91dERpcmVjdGlvbgEABWFwcGx5AQA1KElJSUxhbmRyb2lkL2dyYXBoaWNzL1JlY3Q7TGFuZHJvaWQvZ3JhcGhpY3MvUmVjdDtJKVYBAAF3AQABaAEACWNvbnRhaW5lcgEAF0xhbmRyb2lkL2dyYXBoaWNzL1JlY3Q7AQAHb3V0UmVjdAEANyhJSUlMYW5kcm9pZC9ncmFwaGljcy9SZWN0O0lJTGFuZHJvaWQvZ3JhcGhpY3MvUmVjdDtJKVYBAAR4QWRqAQAEeUFkagEADGFwcGx5RGlzcGxheQEAMyhJTGFuZHJvaWQvZ3JhcGhpY3MvUmVjdDtMYW5kcm9pZC9ncmFwaGljcy9SZWN0O0kpVgEAB2Rpc3BsYXkBAAhpbm91dE9iagEAClNvdXJjZUZpbGUBABJHcmF2aXR5Q29tcGF0LmphdmEMAAkACgcAMgwAEgATDAAXABgMABcAHgwAIQAiBwAzAQBDYW5kcm9pZC9zdXBwb3J0L3Y0L3ZpZXcvR3Jhdml0eUNvbXBhdCRHcmF2aXR5Q29tcGF0SW1wbEplbGx5YmVhbk1yMQEAEGphdmEvbGFuZy9PYmplY3QBADdhbmRyb2lkL3N1cHBvcnQvdjQvdmlldy9HcmF2aXR5Q29tcGF0JEdyYXZpdHlDb21wYXRJbXBsAQARR3Jhdml0eUNvbXBhdEltcGwBADFhbmRyb2lkL3N1cHBvcnQvdjQvdmlldy9HcmF2aXR5Q29tcGF0SmVsbHliZWFuTXIxAQAlYW5kcm9pZC9zdXBwb3J0L3Y0L3ZpZXcvR3Jhdml0eUNvbXBhdAAgAAYABwABAAgAAAAFAAAACQAKAAEACwAAAC8AAQABAAAABSq3AAGxAAAAAgAMAAAABgABAAAAPgANAAAADAABAAAABQAOABEAAAABABIAEwABAAsAAABEAAIAAwAAAAYbHLgAAqwAAAACAAwAAAAGAAEAAABBAA0AAAAgAAMAAAAGAA4AEQAAAAAABgAUABUAAQAAAAYAFgAVAAIAAQAXABgAAQALAAAAdwAGAAcAAAANGxwdGQQZBRUGuAADsQAAAAIADAAAAAoAAgAAAEcADABIAA0AAABIAAcAAAANAA4AEQAAAAAADQAUABUAAQAAAA0AGQAVAAIAAAANABoAFQADAAAADQAbABwABAAAAA0AHQAcAAUAAAANABYAFQAGAAEAFwAeAAEACwAAAI8ACAAJAAAAERscHRkEFQUVBhkHFQi4AASxAAAAAgAMAAAACgACAAAATQAQAE8ADQAAAFwACQAAABEADgARAAAAAAARABQAFQABAAAAEQAZABUAAgAAABEAGgAVAAMAAAARABsAHAAEAAAAEQAfABUABQAAABEAIAAVAAYAAAARAB0AHAAHAAAAEQAWABUACAABACEAIgABAAsAAABfAAQABQAAAAkbLC0VBLgABbEAAAACAAwAAAAKAAIAAABTAAgAVAANAAAANAAFAAAACQAOABEAAAAAAAkAFAAVAAEAAAAJACMAHAACAAAACQAkABwAAwAAAAkAFgAVAAQAAgAlAAAAAgAmABAAAAASAAIABgAtAA8ACAAIAC0AMQYI"

// TestDexEsPresupuestoElBloqueRange: el error del bloque /range se clasifica como presupuesto para
// que `codeItemConPc` suba la base y repita la pasada.
func TestDexEsPresupuestoElBloqueRange(t *testing.T) {
	err := &traduceError{nombre: "x", motivo: "the /range block needs more temporaries than the max stack javap measured"}
	if !esPresupuesto(err) {
		t.Error("el bloque /range no se trata como presupuesto")
	}
}

// TestDexElPresupuestoSubeLaBase: el `.class` de javac de `GravityCompat...apply` necesita más
// temporales que max_stack para el bloque /range; dexarlo tiene que subir la base y terminar. Falla
// si se revierte la clasificación o el pico que anota `tomaBloque`.
func TestDexElPresupuestoSubeLaBase(t *testing.T) {
	datos, err := base64.StdEncoding.DecodeString(gravityJavac)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	clase, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture no se lee: %v", err)
	}
	if _, err := dexDeClases([]classFull{clase}); err != nil {
		t.Fatalf("dexando el .class: %v", err)
	}
}
