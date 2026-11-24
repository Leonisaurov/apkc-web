package main

import (
	"bytes"
	"testing"
)

// Este fichero cierra el plegado de la familia de bits, que era la última frontera conocida del dex de
// ApkShare en `Apk.java`. El caso que lo abrió es real: `PackageManager.GET_ACTIVITIES |
// PackageManager.GET_META_DATA` (1 | 128), en la línea 16 del `src/share/apps/util/Apk.java` del zip,
// que javac 17 `--release 8 -g:lines,source` dobla a `sipush 129` y que nuestro emisor escribía como
// dos cargas y un `ior`. Todo se mide contra javac y, además, contra lo que javac deja en el dex.

// fuentePlegadoDeBits: los dos operandos literales de cada operador de bits, el `long` que promociona
// al otro lado, la constante nombrada por su tipo y el control que no se puede plegar porque el
// operando es una local. Medido con javac 17 `--release 8 -g:lines,source`: `5 | 2` → `iconst_7`,
// `5 & 6` → `iconst_4`, `5 ^ 3` → `bipush 6`, `1L | 128L` y `1 | 128L` → `ldc2_w 129l`.
const fuentePlegadoDeBits = `package p;

final class Limites {
    public static final int UNO = 1;
    public static final int CIENTO_VEINTIOCHO = 128;
}

class Sonda {
    int o() { return 1 | 128; }
    int y() { return 5 & 6; }
    int x() { return 5 ^ 3; }
    long ol() { return 1L | 128L; }
    long om() { return 1 | 128L; }
    int constante() { return Limites.UNO | Limites.CIENTO_VEINTIOCHO; }
    int control(int base) { return base | 128; }
}
`

// opcodesDe: la sucesión de opcodes de un `Code`, sin operandos. Es lo que separa «el mismo programa»
// de «los mismos huecos»: el reparto de locales de un bloque puede diferir del de javac sin que la
// instrucción cambie.
func opcodesDe(t *testing.T, code []byte) []byte {
	t.Helper()
	ins, err := walkJvmCode(code)
	if err != nil {
		t.Fatalf("el Code no se desarma: %v", err)
	}
	out := make([]byte, 0, len(ins))
	for _, in := range ins {
		out = append(out, in.op)
	}
	return out
}

// tieneOpcode: ¿el `Code` lleva alguna vez ese opcode? Sirve para los controles negativos (`ior` es
// 0x80 y no puede aparecer si el `|` se dobló).
func tieneOpcode(t *testing.T, code []byte, op byte) bool {
	t.Helper()
	ins, err := walkJvmCode(code)
	if err != nil {
		t.Fatalf("el Code no se desarma: %v", err)
	}
	for _, in := range ins {
		if in.op == op {
			return true
		}
	}
	return false
}

// llevaSipush: ¿el `Code` carga esa constante con `sipush` (0x11)? Es la forma que javac elige para
// 129, que no cabe en `bipush` (el rango llega a 127) ni en `iconst`.
func llevaSipush(t *testing.T, code []byte, valor int) bool {
	t.Helper()
	ins, err := walkJvmCode(code)
	if err != nil {
		t.Fatalf("el Code no se desarma: %v", err)
	}
	for _, in := range ins {
		if in.op == 0x11 && len(in.ops) == 2 && int(int16(uint16(in.ops[0])<<8|uint16(in.ops[1]))) == valor {
			return true
		}
	}
	return false
}

// TestElPlegadoDeBitsContraJavac17: los siete métodos de la sonda, instrucción a instrucción y con la
// LineNumberTable, contra javac. El `control` demuestra que el plegado no se come un operando que no
// es literal: `base | 128` sigue con su `ior` y es, también, el de javac.
func TestElPlegadoDeBitsContraJavac17(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	fuentes := []fuenteJava{{nombre: "src/p/Sonda.java", contenido: fuentePlegadoDeBits}}

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó la sonda: %v", err)
	}
	raiz := t.TempDir()
	pon(t, raiz, "p/Sonda.java", fuentePlegadoDeBits)
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	for _, nombre := range []string{"o", "y", "x", "ol", "om", "constante", "control"} {
		nuestro := metodoDeClaseEmitida(t, nuestras, "p/Sonda", nombre, "")
		suyo := metodoDeClaseLeida(t, deJavac, "Lp/Sonda;", nombre, "")
		if err := comparaInstrucciones(nuestro.Code, suyo.Code); err != nil {
			t.Fatalf("%s no es el de javac: %v\n%s", nombre, err, volcadoDeClases(nuestras))
		}
		if a, b := lineasDe(nuestro.Lines), lineasDe(suyo.Lines); a != b {
			t.Fatalf("%s mueve la LineNumberTable: %q frente a %q", nombre, a, b)
		}
	}

	// el plegado, además de coincidir con javac, tiene que ser el valor doblado: si el emisor quitara
	// las dos cargas y el `ior` sin poner la constante, el `comparaInstrucciones` de arriba ya lo vería,
	// pero esta comprobación deja escrito qué constante se espera.
	o := metodoDeClaseEmitida(t, nuestras, "p/Sonda", "o", "")
	if tieneOpcode(t, o.Code, 0x80) {
		t.Fatalf("`1 | 128` sigue emitiendo `ior`\n%s", volcadoDeClases(nuestras))
	}
	if !llevaSipush(t, o.Code, 129) {
		t.Fatalf("`1 | 128` no dejó `sipush 129`\n%s", volcadoDeClases(nuestras))
	}
	// el control: el operando que no es literal no se pliega
	control := metodoDeClaseEmitida(t, nuestras, "p/Sonda", "control", "")
	if !tieneOpcode(t, control.Code, 0x80) {
		t.Fatalf("`base | 128` se plegó y no debía: no queda `ior`\n%s", volcadoDeClases(nuestras))
	}
}

// TestElApkDeApkSharePlegaElOrDeLosDosPermisos: el fuente real de `Apk.java` del zip de ApkShare (el
// que `fuentesAppAdapterShare` trae literal) sobre el proyecto de seis clases. El `|` que quedaba
// abierto sale como un solo `sipush 129` y la LineNumberTable del constructor no se mueve: la fila de
// la sentencia sigue apuntando al primer byte de la llamada, que es lo que escribe javac.
//
// El dex del proyecto de seis clases **no** se compara byte a byte aquí: lo compara
// `TestElDexDeLasSeisClasesDeApkShareEsElDeJavac17`, que llegó después de cerrar el reparto de huecos
// de los bloques. Aquí se mide solo el plegado, que es de este fichero.
func TestElApkDeApkSharePlegaElOrDeLosDosPermisos(t *testing.T) {
	jar := jarDelSDK(t)
	javac := javac17(t)
	fuentes := fuentesAppAdapterShare()

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó los fuentes reales de ApkShare: %v", err)
	}
	raiz := t.TempDir()
	for _, f := range fuentes {
		pon(t, raiz, f.nombre[len("src/"):], f.contenido)
	}
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	const desc = "(Landroid/content/pm/PackageManager;Ljava/lang/String;)V"
	nuestro := metodoDeClaseEmitida(t, nuestras, "share/apps/util/Apk", "<init>", desc)
	suyo := metodoDeClaseLeida(t, deJavac, "Lshare/apps/util/Apk;", "<init>", desc)

	if tieneOpcode(t, nuestro.Code, 0x80) {
		t.Fatalf("`GET_ACTIVITIES | GET_META_DATA` sigue emitiendo `ior`\n%s", volcadoDeClases(nuestras))
	}
	if !llevaSipush(t, nuestro.Code, 129) {
		t.Fatalf("el constructor no dobló el `|` a 129\n%s", volcadoDeClases(nuestras))
	}
	// el mismo programa que javac, instrucción a instrucción: ahora también el mismo hueco para
	// `name`, que es lo que cerró el reparto de bloques (ver `javaemisor_ambito_local_test.go`)
	if a, b := string(opcodesDe(t, nuestro.Code)), string(opcodesDe(t, suyo.Code)); a != b {
		t.Fatalf("los opcodes de `Apk.<init>` no son los de javac:\nnuestro %v\njavac   %v", []byte(a), []byte(b))
	}
	if a, b := lineasDe(nuestro.Lines), lineasDe(suyo.Lines); a != b {
		t.Fatalf("el constructor mueve la LineNumberTable: %q frente a %q", a, b)
	}
}

// TestFalsificacionDelPlegadoDeBits: con el interruptor apagado, el `|` de los permisos vuelve a
// salir como las dos cargas y el `or-int`/`ior`, y el constructor deja de ser el de javac. Es la ruta
// nueva y no otra cosa.
func TestFalsificacionDelPlegadoDeBits(t *testing.T) {
	anterior := pliegaBits
	pliegaBits = false
	t.Cleanup(func() { pliegaBits = anterior })

	jar := jarDelSDK(t)
	javac := javac17(t)
	fuentes := fuentesAppAdapterShare()

	nuestras, err := compilarProyectoJava(fuentes, jar)
	if err != nil {
		t.Fatalf("el emisor rechazó los fuentes reales de ApkShare: %v", err)
	}
	raiz := t.TempDir()
	for _, f := range fuentes {
		pon(t, raiz, f.nombre[len("src/"):], f.contenido)
	}
	deJavac := compilaConJavac(t, javac, jar, raiz, fuentes)

	const desc = "(Landroid/content/pm/PackageManager;Ljava/lang/String;)V"
	nuestro := metodoDeClaseEmitida(t, nuestras, "share/apps/util/Apk", "<init>", desc)
	suyo := metodoDeClaseLeida(t, deJavac, "Lshare/apps/util/Apk;", "<init>", desc)

	if !tieneOpcode(t, nuestro.Code, 0x80) {
		t.Fatalf("con el plegado apagado el `|` no se emitió como `ior`\n%s", volcadoDeClases(nuestras))
	}
	if llevaSipush(t, nuestro.Code, 129) {
		t.Fatalf("con el plegado apagado sigue apareciendo la constante doblada 129\n%s", volcadoDeClases(nuestras))
	}
	if err := comparaInstrucciones(nuestro.Code, suyo.Code); err == nil {
		t.Fatalf("con el plegado apagado el constructor sigue siendo el de javac, que es imposible")
	} else {
		t.Logf("diferencia con javac al apagar el plegado: %v", err)
	}
	// y el dex de las seis clases tampoco es el de javac: dos instrucciones de más en el constructor,
	// que es lo que separa el `iconst_1; bipush 128; ior` del `sipush 129`
	nuestroDex := dexDeNuestrasClases(t, nuestras)
	suDex := dexDeLasDeJavac(t, deJavac)
	if bytes.Equal(nuestroDex, suDex) {
		t.Fatalf("el dex con el plegado apagado es el de javac")
	}
	t.Logf("dex con el plegado apagado: nuestro %d bytes, javac %d, y no coinciden byte a byte", len(nuestroDex), len(suDex))
}
