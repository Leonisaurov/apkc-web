package main

// La promoción de `goto`: un `goto` 10t solo alcanza +-127 unidades y, cuando el salto es mayor, hay
// que emitirlo como `goto/16` 0x29 (21t, desplazamiento de 16 bits en la unidad siguiente). Medido
// con `javac`, `d8` y `dexdump` sobre `Largo`, un bucle con doscientas asignaciones: `d8` escribe
// `2900 46fe` en el borde de vuelta, o sea `goto/16` con -442 unidades. La instrucción crece una
// unidad, así que la pasada se repite hasta que ningún `goto` quede fuera de rango.

import (
	"encoding/base64"
	"testing"
)

// largoGotoJavac: `javac 21 --release 8 -g:lines,source Largo.java` y `base64 -w0 Largo.class`.
const largoGotoJavac = "yv66vgAAADQAEAoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQAFTGFyZ28BAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQAFYnVjbGUBAAQoSSlJAQANU3RhY2tNYXBUYWJsZQEAClNvdXJjZUZpbGUBAApMYXJnby5qYXZhACEABwACAAAAAAACAAEABQAGAAEACQAAAB0AAQABAAAABSq3AAGxAAAAAQAKAAAABgABAAAAAQAIAAsADAABAAkAAAY1AAEAAQAAAp0angKahAABhAAChAADhAAEhAAFhAAGhAAHhAABhAAChAADhAAEhAAFhAAGhAAHhAABhAAChAADhAAEhAAFhAAGhAAHhAABhAAChAADhAAEhAAFhAAGhAAHhAABhAAChAADhAAEhAAFhAAGhAAHhAABhAAChAADhAAEhAAFhAAGhAAHhAABhAAChAADhAAEhAAFhAAGhAAHhAABhAAChAADhAAEhAAFhAAGhAAHhAABhAAChAADhAAEhAAFhAAGhAAHhAABhAAChAADhAAEhAAFhAAGhAAHhAABhAAChAADhAAEhAAFhAAGhAAHhAABhAAChAADhAAEhAAFhAAGhAAHhAABhAAChAADhAAEhAAFhAAGhAAHhAABhAAChAADhAAEhAAFhAAGhAAHhAABhAAChAADhAAEhAAFhAAGhAAHhAABhAAChAADhAAEhAAFhAAGhAAHhAABhAAChAADhAAEhAAFhAAGhAAHhAABhAAChAADhAAEhAAFhAAGhAAHhAABhAAChAADhAAEhAAFhAAGhAAHhAABhAAChAADhAAEhAAFhAAGhAAHhAABhAAChAADhAAEhAAFhAAGhAAHhAABhAAChAADhAAEhAAFhAAGhAAHhAABhAAChAADhAAEhAAFhAAGhAAHhAABhAAChAADhAAEhAAFhAAGhAAHhAABhAAChAADhAAEhAAFhAAGhAAHhAABhAAChAADhAAEhAAFhAAGhAAHhAABhAAChAADhAAEhAAFhAAGhAAHhAABhAAChAADhAAEhAAFhAAGhAAHhAABhAAChAADhAAEhAAFhAAGhAAHhAABhAAChAADhAAEhAAFhAAGhAAHhAABhAAChAADhAAEhAAFhAAGhAAHhAABhAAChAADp/1oGqwAAAACAAoAAAN6AN4AAAADAAQABAAHAAUACgAGAA0ABwAQAAgAEwAJABYACgAZAAsAHAAMAB8ADQAiAA4AJQAPACgAEAArABEALgASADEAEwA0ABQANwAVADoAFgA9ABcAQAAYAEMAGQBGABoASQAbAEwAHABPAB0AUgAeAFUAHwBYACAAWwAhAF4AIgBhACMAZAAkAGcAJQBqACYAbQAnAHAAKABzACkAdgAqAHkAKwB8ACwAfwAtAIIALgCFAC8AiAAwAIsAMQCOADIAkQAzAJQANACXADUAmgA2AJ0ANwCgADgAowA5AKYAOgCpADsArAA8AK8APQCyAD4AtQA/ALgAQAC7AEEAvgBCAMEAQwDEAEQAxwBFAMoARgDNAEcA0ABIANMASQDWAEoA2QBLANwATADfAE0A4gBOAOUATwDoAFAA6wBRAO4AUgDxAFMA9ABUAPcAVQD6AFYA/QBXAQAAWAEDAFkBBgBaAQkAWwEMAFwBDwBdARIAXgEVAF8BGABgARsAYQEeAGIBIQBjASQAZAEnAGUBKgBmAS0AZwEwAGgBMwBpATYAagE5AGsBPABsAT8AbQFCAG4BRQBvAUgAcAFLAHEBTgByAVEAcwFUAHQBVwB1AVoAdgFdAHcBYAB4AWMAeQFmAHoBaQB7AWwAfAFvAH0BcgB+AXUAfwF4AIABewCBAX4AggGBAIMBhACEAYcAhQGKAIYBjQCHAZAAiAGTAIkBlgCKAZkAiwGcAIwBnwCNAaIAjgGlAI8BqACQAasAkQGuAJIBsQCTAbQAlAG3AJUBugCWAb0AlwHAAJgBwwCZAcYAmgHJAJsBzACcAc8AnQHSAJ4B1QCfAdgAoAHbAKEB3gCiAeEAowHkAKQB5wClAeoApgHtAKcB8ACoAfMAqQH2AKoB+QCrAfwArAH/AK0CAgCuAgUArwIIALACCwCxAg4AsgIRALMCFAC0AhcAtQIaALYCHQC3AiAAuAIjALkCJgC6AikAuwIsALwCLwC9AjIAvgI1AL8COADAAjsAwQI+AMICQQDDAkQAxAJHAMUCSgDGAk0AxwJQAMgCUwDJAlYAygJZAMsCXADMAl8AzQJiAM4CZQDPAmgA0AJrANECbgDSAnEA0wJ0ANQCdwDVAnoA1gJ9ANcCgADYAoMA2QKGANoCiQDbAowA3AKPAN0CkgDeApUA3wKbAOEADQAAAAYAAgD7ApoAAQAOAAAAAgAP"

func claseLargoGotoDeJavac(t *testing.T) classFull {
	t.Helper()
	datos, err := base64.StdEncoding.DecodeString(largoGotoJavac)
	if err != nil {
		t.Fatalf("el fixture no es base64: %v", err)
	}
	cf, err := readClassFull(datos)
	if err != nil {
		t.Fatalf("el fixture de javac no se lee: %v", err)
	}
	return cf
}

// TestDexElGotoLargoSePromueveAGoto16: el `goto` de vuelta del bucle no cabe en 10t; el dexer tiene
// que emitirlo como `goto/16` 0x29 con un desplazamiento fuera de +-127. Si se quita la promoción,
// `dexDeClases` falla con el error del 10t y esta prueba muere antes de mirar el opcode.
func TestDexElGotoLargoSePromueveAGoto16(t *testing.T) {
	clase := claseLargoGotoDeJavac(t)
	if _, err := dexDeClases([]classFull{clase}); err != nil {
		t.Fatalf("dexando el .class de javac: %v", err)
	}
	td := newDexTables()
	if err := td.collectClasses([]classFull{clase}); err != nil {
		t.Fatalf("internando: %v", err)
	}
	td.orderTables()
	m, ok := buscaMetodo(clase, "bucle")
	if !ok {
		t.Fatal("bucle: no está en el fixture de javac")
	}
	ci, err := td.codeItemDe(clase, m)
	if err != nil {
		t.Fatalf("bucle: dexando: %v", err)
	}
	encontrado := false
	for i := 0; i+1 < len(ci.insns); i++ {
		if byte(ci.insns[i]) != opGoto16 {
			continue
		}
		desplazamiento := int(int16(ci.insns[i+1]))
		if desplazamiento < -127 || desplazamiento > 127 {
			encontrado = true
			t.Logf("goto/16 en la unidad %d con desplazamiento %d", i, desplazamiento)
			break
		}
	}
	if !encontrado {
		t.Errorf("no hay un goto/16 fuera de +-127 en %04x", ci.insns)
	}
}
