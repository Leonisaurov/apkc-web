package main

// Los atajos de carga de local (`aload_0`, `aload_1`, `iload_0`, `lload_0`) cuando el hueco cae por
// encima de v15: tienen que bajar el local a un temporal con `move*/from16`, igual que el camino
// genérico (`cargaLocal`/`bajaAlto`), no fallar en el nibble. Lo destapó `BackStackRecord.dump` del
// android-support-v4.jar, con `aload_1` sobre v30.

import "testing"

// metodoAlto: un método con `LocalCount` grande para que el hueco 0 o 1 caiga por encima de v15, con
// la base ya fijada (pasada final, `techo`) y el cuerpo indicado.
func metodoAlto(codigo []byte, argCount int) *traduce {
	m := classMethod{
		Name:       "alto",
		Desc:       "()V",
		HasCode:    true,
		MaxStack:   16,
		LocalCount: 20,
		ArgCount:   argCount,
		Code:       codigo,
	}
	// base 16: el hueco 0 (argumento) cae en 16+20-argCount, siempre por encima de v15
	return &traduce{m: m, locales: m.LocalCount, base: 16, techo: true}
}

// TestDexLasCargasDeLocalBajanUnHuecoAlto: `aload_0`, `aload_1`, `iload_0` y `lload_0` emiten su
// `move*/from16` desde el hueco alto a un temporal bajo. Si se revierte el atajo (volver a
// `p.local(n)` + `nibble`), `recorre` falla en el nibble y la prueba lo dice.
func TestDexLasCargasDeLocalBajanUnHuecoAlto(t *testing.T) {
	casos := []struct {
		nombre string
		codigo []byte // carga; retorno
		args   int
		hueco  int
		op     byte // move*/from16 esperado
	}{
		{"aload_0", []byte{0x2a, 0xb0}, 1, 0, 0x08}, // aload_0; areturn
		{"aload_1", []byte{0x2b, 0xb0}, 2, 1, 0x08}, // aload_1; areturn
		{"iload_0", []byte{0x1a, 0xac}, 1, 0, 0x02}, // iload_0; ireturn
		{"lload_0", []byte{0x1e, 0xad}, 1, 0, 0x05}, // lload_0; lreturn
	}
	for _, c := range casos {
		p := metodoAlto(c.codigo, c.args)
		// el hueco antes de traducir: `bajaAlto` lo renombra al temporal y `local` ya devolvería ese
		origen := p.local(c.hueco)
		if err := p.recorre(); err != nil {
			t.Errorf("%s: %v", c.nombre, err)
			continue
		}
		if len(p.w) < 2 || byte(p.w[0]) != c.op {
			t.Errorf("%s: esperaba %#02x (move*/from16) al principio, salió %04x", c.nombre, c.op, p.w)
			continue
		}
		if p.w[1] != uint16(origen) {
			t.Errorf("%s: el move viene del registro %d y el hueco %d está en %d",
				c.nombre, p.w[1], c.hueco, origen)
		}
	}
}
