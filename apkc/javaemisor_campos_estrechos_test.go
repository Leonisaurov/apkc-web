package main

// Los accesos a campos de una palabra que no son `int` ni `float`, medidos con `javac`, `d8` y
// `dexdump` sobre `Ang`: el `boolean`, el `byte`, el `char` y el `short` tienen opcode propio tanto
// en la forma de instancia (22c) como en la estática (21c). `ActionBarDrawerToggle$SlideDrawable`
// fue el primero que destapó el límite, con un `putfield` de tipo `Z`.

import (
	"encoding/base64"
	"testing"
)

// angJavac: `javac 21 --release 8 -g:lines,source Ang.java` y `base64 -w0 Ang.class`. Cuatro campos
// y cuatro estáticos, con su lector y su escritor por tipo.
const angJavac = "yv66vgAAADQAQQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCQAIAAkHAAoMAAsADAEAA0FuZwEAAnpiAQABWgkACAAODAAPABABAAJiYgEAAUIJAAgAEgwAEwAUAQACY2IBAAFDCQAIABYMABcAGAEAAnNiAQABUwkACAAaDAAbAAwBAAJ6cwkACAAdDAAeABABAAJicwkACAAgDAAhABQBAAJjcwkACAAjDAAkABgBAAJzcwEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAAJyYgEAAygpWgEAAndiAQAEKFopVgEAA3JieQEAAygpQgEAA3dieQEABChCKVYBAAJyYwEAAygpQwEAAndjAQAEKEMpVgEAAnJzAQADKClTAQACd3MBAAQoUylWAQADcnpzAQADd3pzAQAEcnpieQEABHd6YnkBAANyemMBAAN3emMBAARyenMyAQAEd3pzMgEAClNvdXJjZUZpbGUBAAhBbmcuamF2YQAhAAgAAgAAAAgAAAALAAwAAAAAAA8AEAAAAAAAEwAUAAAAAAAXABgAAAAIABsADAAAAAgAHgAQAAAACAAhABQAAAAIACQAGAAAABEAAQAFAAYAAQAlAAAAHQABAAEAAAAFKrcAAbEAAAABACYAAAAGAAEAAAABAAAAJwAoAAEAJQAAAB0AAQABAAAABSq0AAesAAAAAQAmAAAABgABAAAABAAAACkAKgABACUAAAAeAAIAAgAAAAYqG7UAB7EAAAABACYAAAAGAAEAAAAFAAAAKwAsAAEAJQAAAB0AAQABAAAABSq0AA2sAAAAAQAmAAAABgABAAAABgAAAC0ALgABACUAAAAeAAIAAgAAAAYqG7UADbEAAAABACYAAAAGAAEAAAAHAAAALwAwAAEAJQAAAB0AAQABAAAABSq0ABGsAAAAAQAmAAAABgABAAAACAAAADEAMgABACUAAAAeAAIAAgAAAAYqG7UAEbEAAAABACYAAAAGAAEAAAAJAAAAMwA0AAEAJQAAAB0AAQABAAAABSq0ABWsAAAAAQAmAAAABgABAAAACgAAADUANgABACUAAAAeAAIAAgAAAAYqG7UAFbEAAAABACYAAAAGAAEAAAALAAgANwAoAAEAJQAAABwAAQAAAAAABLIAGawAAAABACYAAAAGAAEAAAAMAAgAOAAqAAEAJQAAAB0AAQABAAAABRqzABmxAAAAAQAmAAAABgABAAAADQAIADkALAABACUAAAAcAAEAAAAAAASyABysAAAAAQAmAAAABgABAAAADgAIADoALgABACUAAAAdAAEAAQAAAAUaswAcsQAAAAEAJgAAAAYAAQAAAA8ACAA7ADAAAQAlAAAAHAABAAAAAAAEsgAfrAAAAAEAJgAAAAYAAQAAABAACAA8ADIAAQAlAAAAHQABAAEAAAAFGrMAH7EAAAABACYAAAAGAAEAAAARAAgAPQA0AAEAJQAAABwAAQAAAAAABLIAIqwAAAABACYAAAAGAAEAAAASAAgAPgA2AAEAJQAAAB0AAQABAAAABRqzACKxAAAAAQAmAAAABgABAAAAEwABAD8AAAACAEA="

func claseAngDeJavac(t *testing.T) classFull {
	t.Helper()
	datos, err := base64.StdEncoding.DecodeString(angJavac)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	cf, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture de javac no se lee: %v", err)
	}
	return cf
}

// TestDexCamposEstrechosDeUnClassDeJar: cada lector y escritor del `.class` de javac sale con el
// opcode 22c/21c que mide `d8` para su tipo. Falla si desaparece una fila de
// `opcodesDeCampoInstancia`/`opcodesDeCampoEstatico`: el `accesoCampo`/`accesoEstatico` la rechaza
// por nombre.
func TestDexCamposEstrechosDeUnClassDeJar(t *testing.T) {
	clase := claseAngDeJavac(t)
	if _, err := dexDeClases([]classFull{clase}); err != nil {
		t.Fatalf("dexando el .class de javac: %v", err)
	}
	td := newDexTables()
	if err := td.collectClasses([]classFull{clase}); err != nil {
		t.Fatalf("internando: %v", err)
	}
	td.orderTables()
	sondas := map[string]byte{
		// instancia: iget 0x55-0x58 y iput 0x5c-0x5f
		"rb": opIgetBoolean, "rby": opIgetByte, "rc": opIgetChar, "rs": opIgetShort,
		"wb": opIputBoolean, "wby": opIputByte, "wc": opIputChar, "ws": opIputShort,
		// estáticos: sget 0x63-0x66 y sput 0x6a-0x6d
		"rzs": opSgetBoolean, "rzby": opSgetByte, "rzc": opSgetChar, "rzs2": opSgetShort,
		"wzs": opSputBoolean, "wzby": opSputByte, "wzc": opSputChar, "wzs2": opSputShort,
	}
	for nombre, opcode := range sondas {
		m, ok := buscaMetodo(clase, nombre)
		if !ok {
			t.Fatalf("%s: no está en el fixture de javac", nombre)
		}
		ci, err := td.codeItemDe(clase, m)
		if err != nil {
			t.Fatalf("%s: dexando: %v", nombre, err)
		}
		visto := false
		for _, u := range ci.insns {
			if byte(u) == opcode {
				visto = true
				break
			}
		}
		if !visto {
			t.Errorf("%s: no aparece el opcode %#02x en %04x", nombre, opcode, ci.insns)
		}
	}
}
