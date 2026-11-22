package main

import (
	"bytes"
	"strings"
	"testing"
)

// Sondas de la resolución de sobrecargas medida con javac 17.0.21 (`--release 8 -g:lines,source`).
// Las fuentes van en una línea, como las demás sondas, para que `LineNumberTable` coincida, y el
// `.class` de javac viaja en base64 para que la prueba no necesite un javac instalado. El oráculo
// compara el dex de los dos `.class` byte a byte.
//
// Lo que cada caso cierra:
//
//	null implícito / explícito / estático cualificado: `f(null)` con `f(String)` y `f(Object)` elige
//	    `(Ljava/lang/String;)V` (JVMS 15.12.2.2 y 15.12.2.5: el `null` cabe en cualquier referencia y
//	    la especificidad se queda con la más estrecha; el tipo del `null` es `tipoNulo`, que no
//	    coincide con ningún descriptor real).
//	fases 1 y 2: `f(1)` con `f(Integer)` y `f(long)` elige `(J)V` y `f('a')` con `f(Character)` y
//	    `f(int)` elige `(I)V`, porque en la fase estricta no hay boxing y el ensanchamiento primitivo sí.
//	desenvuelto: `g(i)` con `i` un `Integer` y `g(int)` escribe `Integer.intValue:()I`; hacia un
//	    `long`, además `i2l`.
//	`new` como argumento: `f(new S())` con `f(Object)` estima `LS;` y baja `new`/`dup`/`<init>`.
const (
	sobreNuloImplicitoFuente     = `public class S { static void f(String x){} static void f(Object x){} static void t(){ f(null); } }`
	sobreNuloExplicitoFuente     = `public class S { void f(String x){} void f(Object x){} void t(S s){ s.f(null); } }`
	sobreNuloEstaticoFuente      = `public class S { static void f(String x){} static void f(Object x){} static void t(){ S.f(null); } }`
	sobreIntegerLargoFuente      = `public class S { static void f(Integer x){} static void f(long x){} static void t(){ f(1); } }`
	sobreCaracterEnteroFuente    = `public class S { static void f(Character c){} static void f(int i){} static void t(){ f('a'); } }`
	sobreDesenvueltoFuente       = `public class S { static void g(int x){} static void t(Integer i){ g(i); } }`
	sobreDesenvueltoAnchoFuente  = `public class S { static void h(long x){} static void t(Integer i){ h(i); } }`
	sobreNuevoArgumentoFuente    = `public class S { static void f(Object o){} static void t(){ f(new S()); } }`
	sobreEnteroLargoFuente       = `public class S { static void f(int x){} static void f(long x){} static void t(){ f(1); } }`
	sobreLargoFlotanteFuente     = `public class S { static void f(long x){} static void f(float x){} static void t(){ f(1); } }`
	sobreDesenvueltoExpFuente    = `public class S { void g(int x){} void t(S s, Integer i){ s.g(i); } }`
	sobreDesenvueltoCualFuente   = `public class S { static void g(int x){} static void t(Integer i){ S.g(i); } }`
	sobreNuloSolteroFuente       = `public class S { static void f(String x){} static void t(){ f(null); } }`
	sobreNuloAmbiguoFuente       = `public class S { static void f(String x){} static void f(Integer x){} static void t(){ f(null); } }`
	sobreNuloVarargsFuente       = `public class S { static void f(String... x){} static void t(){ f(null); } }`
	sobreNuloVarargsTipFuente    = `public class S { void f(String... x){} void t(S s){ s.f(null); } }`
	sobreNuloVarargsIntFuente    = `public class S { static void f(int... x){} static void t(){ f(null); } }`
	sobreArregloCovarianteFuente = `public class S { static void f(Object[] x){} static void f(Object x){} static void t(){ f(new String[1]); } }`
	sobreNuloArregloFuente       = `public class S { static void f(Object[] x){} static void f(Object x){} static void t(){ f(null); } }`
	sobreNuevoArregloFuente      = `public class S { static void f(Object o){} static void t(){ f(new int[]{1,2}); } }`
)

const (
	sobreNuloImplicitoJavac     = "yv66vgAAADQAEwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEAAVMBAAFmAQAVKExqYXZhL2xhbmcvU3RyaW5nOylWAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAFShMamF2YS9sYW5nL09iamVjdDspVgEAAXQBAApTb3VyY2VGaWxlAQAGUy5qYXZhACEACAACAAAAAAAEAAEABQAGAAEADQAAAB0AAQABAAAABSq3AAGxAAAAAQAOAAAABgABAAAAAQAIAAsADAABAA0AAAAZAAAAAQAAAAGxAAAAAQAOAAAABgABAAAAAQAIAAsADwABAA0AAAAZAAAAAQAAAAGxAAAAAQAOAAAABgABAAAAAQAIABAABgABAA0AAAAdAAEAAAAAAAUBuAAHsQAAAAEADgAAAAYAAQAAAAEAAQARAAAAAgAS"
	sobreNuloExplicitoJavac     = "yv66vgAAADQAFAoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEAAVMBAAFmAQAVKExqYXZhL2xhbmcvU3RyaW5nOylWAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAFShMamF2YS9sYW5nL09iamVjdDspVgEAAXQBAAYoTFM7KVYBAApTb3VyY2VGaWxlAQAGUy5qYXZhACEACAACAAAAAAAEAAEABQAGAAEADQAAAB0AAQABAAAABSq3AAGxAAAAAQAOAAAABgABAAAAAQAAAAsADAABAA0AAAAZAAAAAgAAAAGxAAAAAQAOAAAABgABAAAAAQAAAAsADwABAA0AAAAZAAAAAgAAAAGxAAAAAQAOAAAABgABAAAAAQAAABAAEQABAA0AAAAeAAIAAgAAAAYrAbYAB7EAAAABAA4AAAAGAAEAAAABAAEAEgAAAAIAEw=="
	sobreNuloEstaticoJavac      = "yv66vgAAADQAEwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEAAVMBAAFmAQAVKExqYXZhL2xhbmcvU3RyaW5nOylWAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAFShMamF2YS9sYW5nL09iamVjdDspVgEAAXQBAApTb3VyY2VGaWxlAQAGUy5qYXZhACEACAACAAAAAAAEAAEABQAGAAEADQAAAB0AAQABAAAABSq3AAGxAAAAAQAOAAAABgABAAAAAQAIAAsADAABAA0AAAAZAAAAAQAAAAGxAAAAAQAOAAAABgABAAAAAQAIAAsADwABAA0AAAAZAAAAAQAAAAGxAAAAAQAOAAAABgABAAAAAQAIABAABgABAA0AAAAdAAEAAAAAAAUBuAAHsQAAAAEADgAAAAYAAQAAAAEAAQARAAAAAgAS"
	sobreIntegerLargoJavac      = "yv66vgAAADQAEwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEAAVMBAAFmAQAEKEopVgEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBABYoTGphdmEvbGFuZy9JbnRlZ2VyOylWAQABdAEAClNvdXJjZUZpbGUBAAZTLmphdmEAIQAIAAIAAAAAAAQAAQAFAAYAAQANAAAAHQABAAEAAAAFKrcAAbEAAAABAA4AAAAGAAEAAAABAAgACwAPAAEADQAAABkAAAABAAAAAbEAAAABAA4AAAAGAAEAAAABAAgACwAMAAEADQAAABkAAAACAAAAAbEAAAABAA4AAAAGAAEAAAABAAgAEAAGAAEADQAAAB0AAgAAAAAABQq4AAexAAAAAQAOAAAABgABAAAAAQABABEAAAACABI="
	sobreCaracterEnteroJavac    = "yv66vgAAADQAEwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEAAVMBAAFmAQAEKEkpVgEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBABgoTGphdmEvbGFuZy9DaGFyYWN0ZXI7KVYBAAF0AQAKU291cmNlRmlsZQEABlMuamF2YQAhAAgAAgAAAAAABAABAAUABgABAA0AAAAdAAEAAQAAAAUqtwABsQAAAAEADgAAAAYAAQAAAAEACAALAA8AAQANAAAAGQAAAAEAAAABsQAAAAEADgAAAAYAAQAAAAEACAALAAwAAQANAAAAGQAAAAEAAAABsQAAAAEADgAAAAYAAQAAAAEACAAQAAYAAQANAAAAHgABAAAAAAAGEGG4AAexAAAAAQAOAAAABgABAAAAAQABABEAAAACABI="
	sobreDesenvueltoJavac       = "yv66vgAAADQAGQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEAEWphdmEvbGFuZy9JbnRlZ2VyAQAIaW50VmFsdWUBAAMoKUkKAA4ADwcAEAwAEQASAQABUwEAAWcBAAQoSSlWAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAAXQBABYoTGphdmEvbGFuZy9JbnRlZ2VyOylWAQAKU291cmNlRmlsZQEABlMuamF2YQAhAA4AAgAAAAAAAwABAAUABgABABMAAAAdAAEAAQAAAAUqtwABsQAAAAEAFAAAAAYAAQAAAAEACAARABIAAQATAAAAGQAAAAEAAAABsQAAAAEAFAAAAAYAAQAAAAEACAAVABYAAQATAAAAIAABAAEAAAAIKrYAB7gADbEAAAABABQAAAAGAAEAAAABAAEAFwAAAAIAGA=="
	sobreDesenvueltoAnchoJavac  = "yv66vgAAADQAGQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEAEWphdmEvbGFuZy9JbnRlZ2VyAQAIaW50VmFsdWUBAAMoKUkKAA4ADwcAEAwAEQASAQABUwEAAWgBAAQoSilWAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAAXQBABYoTGphdmEvbGFuZy9JbnRlZ2VyOylWAQAKU291cmNlRmlsZQEABlMuamF2YQAhAA4AAgAAAAAAAwABAAUABgABABMAAAAdAAEAAQAAAAUqtwABsQAAAAEAFAAAAAYAAQAAAAEACAARABIAAQATAAAAGQAAAAIAAAABsQAAAAEAFAAAAAYAAQAAAAEACAAVABYAAQATAAAAIQACAAEAAAAJKrYAB4W4AA2xAAAAAQAUAAAABgABAAAAAQABABcAAAACABg="
	sobreNuevoArgumentoJavac    = "yv66vgAAADQAEwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQABUwoABwADCgAHAAsMAAwADQEAAWYBABUoTGphdmEvbGFuZy9PYmplY3Q7KVYBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQABdAEAClNvdXJjZUZpbGUBAAZTLmphdmEAIQAHAAIAAAAAAAMAAQAFAAYAAQAOAAAAHQABAAEAAAAFKrcAAbEAAAABAA8AAAAGAAEAAAABAAgADAANAAEADgAAABkAAAABAAAAAbEAAAABAA8AAAAGAAEAAAABAAgAEAAGAAEADgAAACMAAgAAAAAAC7sAB1m3AAm4AAqxAAAAAQAPAAAABgABAAAAAQABABEAAAACABI="
	sobreEnteroLargoJavac       = "yv66vgAAADQAEwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEAAVMBAAFmAQAEKEkpVgEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAAQoSilWAQABdAEAClNvdXJjZUZpbGUBAAZTLmphdmEAIQAIAAIAAAAAAAQAAQAFAAYAAQANAAAAHQABAAEAAAAFKrcAAbEAAAABAA4AAAAGAAEAAAABAAgACwAMAAEADQAAABkAAAABAAAAAbEAAAABAA4AAAAGAAEAAAABAAgACwAPAAEADQAAABkAAAACAAAAAbEAAAABAA4AAAAGAAEAAAABAAgAEAAGAAEADQAAAB0AAQAAAAAABQS4AAexAAAAAQAOAAAABgABAAAAAQABABEAAAACABI="
	sobreLargoFlotanteJavac     = "yv66vgAAADQAEwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEAAVMBAAFmAQAEKEopVgEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAAQoRilWAQABdAEAClNvdXJjZUZpbGUBAAZTLmphdmEAIQAIAAIAAAAAAAQAAQAFAAYAAQANAAAAHQABAAEAAAAFKrcAAbEAAAABAA4AAAAGAAEAAAABAAgACwAMAAEADQAAABkAAAACAAAAAbEAAAABAA4AAAAGAAEAAAABAAgACwAPAAEADQAAABkAAAABAAAAAbEAAAABAA4AAAAGAAEAAAABAAgAEAAGAAEADQAAAB0AAgAAAAAABQq4AAexAAAAAQAOAAAABgABAAAAAQABABEAAAACABI="
	sobreDesenvueltoExpJavac    = "yv66vgAAADQAGQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEAEWphdmEvbGFuZy9JbnRlZ2VyAQAIaW50VmFsdWUBAAMoKUkKAA4ADwcAEAwAEQASAQABUwEAAWcBAAQoSSlWAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAAXQBABkoTFM7TGphdmEvbGFuZy9JbnRlZ2VyOylWAQAKU291cmNlRmlsZQEABlMuamF2YQAhAA4AAgAAAAAAAwABAAUABgABABMAAAAdAAEAAQAAAAUqtwABsQAAAAEAFAAAAAYAAQAAAAEAAAARABIAAQATAAAAGQAAAAIAAAABsQAAAAEAFAAAAAYAAQAAAAEAAAAVABYAAQATAAAAIQACAAMAAAAJKyy2AAe2AA2xAAAAAQAUAAAABgABAAAAAQABABcAAAACABg="
	sobreDesenvueltoCualJavac   = "yv66vgAAADQAGQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEAEWphdmEvbGFuZy9JbnRlZ2VyAQAIaW50VmFsdWUBAAMoKUkKAA4ADwcAEAwAEQASAQABUwEAAWcBAAQoSSlWAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAAXQBABYoTGphdmEvbGFuZy9JbnRlZ2VyOylWAQAKU291cmNlRmlsZQEABlMuamF2YQAhAA4AAgAAAAAAAwABAAUABgABABMAAAAdAAEAAQAAAAUqtwABsQAAAAEAFAAAAAYAAQAAAAEACAARABIAAQATAAAAGQAAAAEAAAABsQAAAAEAFAAAAAYAAQAAAAEACAAVABYAAQATAAAAIAABAAEAAAAIKrYAB7gADbEAAAABABQAAAAGAAEAAAABAAEAFwAAAAIAGA=="
	sobreNuloSolteroJavac       = "yv66vgAAADQAEgoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEAAVMBAAFmAQAVKExqYXZhL2xhbmcvU3RyaW5nOylWAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAAXQBAApTb3VyY2VGaWxlAQAGUy5qYXZhACEACAACAAAAAAADAAEABQAGAAEADQAAAB0AAQABAAAABSq3AAGxAAAAAQAOAAAABgABAAAAAQAIAAsADAABAA0AAAAZAAAAAQAAAAGxAAAAAQAOAAAABgABAAAAAQAIAA8ABgABAA0AAAAdAAEAAAAAAAUBuAAHsQAAAAEADgAAAAYAAQAAAAEAAQAQAAAAAgAR"
	sobreNuloVarargsJavac       = "yv66vgAAADQAEgoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEAAVMBAAFmAQAWKFtMamF2YS9sYW5nL1N0cmluZzspVgEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAAF0AQAKU291cmNlRmlsZQEABlMuamF2YQAhAAgAAgAAAAAAAwABAAUABgABAA0AAAAdAAEAAQAAAAUqtwABsQAAAAEADgAAAAYAAQAAAAEAiAALAAwAAQANAAAAGQAAAAEAAAABsQAAAAEADgAAAAYAAQAAAAEACAAPAAYAAQANAAAAHQABAAAAAAAFAbgAB7EAAAABAA4AAAAGAAEAAAABAAEAEAAAAAIAEQ=="
	sobreNuloVarargsTipJavac    = "yv66vgAAADQAEwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEAAVMBAAFmAQAWKFtMamF2YS9sYW5nL1N0cmluZzspVgEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBAAF0AQAGKExTOylWAQAKU291cmNlRmlsZQEABlMuamF2YQAhAAgAAgAAAAAAAwABAAUABgABAA0AAAAdAAEAAQAAAAUqtwABsQAAAAEADgAAAAYAAQAAAAEAgAALAAwAAQANAAAAGQAAAAIAAAABsQAAAAEADgAAAAYAAQAAAAEAAAAPABAAAQANAAAAHgACAAIAAAAGKwG2AAexAAAAAQAOAAAABgABAAAAAQABABEAAAACABI="
	sobreNuloVarargsIntJavac    = "yv66vgAAADQAEgoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEAAVMBAAFmAQAFKFtJKVYBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQABdAEAClNvdXJjZUZpbGUBAAZTLmphdmEAIQAIAAIAAAAAAAMAAQAFAAYAAQANAAAAHQABAAEAAAAFKrcAAbEAAAABAA4AAAAGAAEAAAABAIgACwAMAAEADQAAABkAAAABAAAAAbEAAAABAA4AAAAGAAEAAAABAAgADwAGAAEADQAAAB0AAQAAAAAABQG4AAexAAAAAQAOAAAABgABAAAAAQABABAAAAACABE="
	sobreArregloCovarianteJavac = "yv66vgAAADQAFQoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWBwAIAQAQamF2YS9sYW5nL1N0cmluZwoACgALBwAMDAANAA4BAAFTAQABZgEAFihbTGphdmEvbGFuZy9PYmplY3Q7KVYBAARDb2RlAQAPTGluZU51bWJlclRhYmxlAQAVKExqYXZhL2xhbmcvT2JqZWN0OylWAQABdAEAClNvdXJjZUZpbGUBAAZTLmphdmEAIQAKAAIAAAAAAAQAAQAFAAYAAQAPAAAAHQABAAEAAAAFKrcAAbEAAAABABAAAAAGAAEAAAABAAgADQAOAAEADwAAABkAAAABAAAAAbEAAAABABAAAAAGAAEAAAABAAgADQARAAEADwAAABkAAAABAAAAAbEAAAABABAAAAAGAAEAAAABAAgAEgAGAAEADwAAACAAAQAAAAAACAS9AAe4AAmxAAAAAQAQAAAABgABAAAAAQABABMAAAACABQ="
	sobreNuloArregloJavac       = "yv66vgAAADQAEwoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEAAVMBAAFmAQAWKFtMamF2YS9sYW5nL09iamVjdDspVgEABENvZGUBAA9MaW5lTnVtYmVyVGFibGUBABUoTGphdmEvbGFuZy9PYmplY3Q7KVYBAAF0AQAKU291cmNlRmlsZQEABlMuamF2YQAhAAgAAgAAAAAABAABAAUABgABAA0AAAAdAAEAAQAAAAUqtwABsQAAAAEADgAAAAYAAQAAAAEACAALAAwAAQANAAAAGQAAAAEAAAABsQAAAAEADgAAAAYAAQAAAAEACAALAA8AAQANAAAAGQAAAAEAAAABsQAAAAEADgAAAAYAAQAAAAEACAAQAAYAAQANAAAAHQABAAAAAAAFAbgAB7EAAAABAA4AAAAGAAEAAAABAAEAEQAAAAIAEg=="
	sobreNuevoArregloJavac      = "yv66vgAAADQAEgoAAgADBwAEDAAFAAYBABBqYXZhL2xhbmcvT2JqZWN0AQAGPGluaXQ+AQADKClWCgAIAAkHAAoMAAsADAEAAVMBAAFmAQAVKExqYXZhL2xhbmcvT2JqZWN0OylWAQAEQ29kZQEAD0xpbmVOdW1iZXJUYWJsZQEAAXQBAApTb3VyY2VGaWxlAQAGUy5qYXZhACEACAACAAAAAAADAAEABQAGAAEADQAAAB0AAQABAAAABSq3AAGxAAAAAQAOAAAABgABAAAAAQAIAAsADAABAA0AAAAZAAAAAQAAAAGxAAAAAQAOAAAABgABAAAAAQAIAA8ABgABAA0AAAAnAAQAAAAAAA8FvApZAwRPWQQFT7gAB7EAAAABAA4AAAAGAAEAAAABAAEAEAAAAAIAEQ=="
)

// TestFasesDeLaSobrecarga: los cinco fallos de la segunda auditoría adversarial, cada uno contra el
// `.class` de javac 17.0.21. El dex de los dos `.class` sale byte a byte o la prueba falla.
func TestFasesDeLaSobrecarga(t *testing.T) {
	casos := []struct{ nombre, fuente, javac string }{
		{"nulo_implicito", sobreNuloImplicitoFuente, sobreNuloImplicitoJavac},
		{"nulo_receptor_explicito", sobreNuloExplicitoFuente, sobreNuloExplicitoJavac},
		{"nulo_estatico_cualificado", sobreNuloEstaticoFuente, sobreNuloEstaticoJavac},
		{"integer_vs_largo", sobreIntegerLargoFuente, sobreIntegerLargoJavac},
		{"caracter_vs_entero", sobreCaracterEnteroFuente, sobreCaracterEnteroJavac},
		{"desenvuelto", sobreDesenvueltoFuente, sobreDesenvueltoJavac},
		{"desenvuelto_y_ensanchamiento", sobreDesenvueltoAnchoFuente, sobreDesenvueltoAnchoJavac},
		{"desenvuelto_receptor_explicito", sobreDesenvueltoExpFuente, sobreDesenvueltoExpJavac},
		{"desenvuelto_estatico_cualificado", sobreDesenvueltoCualFuente, sobreDesenvueltoCualJavac},
		{"nuevo_como_argumento", sobreNuevoArgumentoFuente, sobreNuevoArgumentoJavac},
		{"entero_vs_largo", sobreEnteroLargoFuente, sobreEnteroLargoJavac},
		{"largo_vs_flotante", sobreLargoFlotanteFuente, sobreLargoFlotanteJavac},
		{"nulo_con_una_sola_candidata", sobreNuloSolteroFuente, sobreNuloSolteroJavac},
		{"nulo_con_varargs", sobreNuloVarargsFuente, sobreNuloVarargsJavac},
		{"nulo_con_varargs_y_receptor", sobreNuloVarargsTipFuente, sobreNuloVarargsTipJavac},
		{"nulo_con_varargs_primitivo", sobreNuloVarargsIntFuente, sobreNuloVarargsIntJavac},
		{"arreglo_covariante", sobreArregloCovarianteFuente, sobreArregloCovarianteJavac},
		{"nulo_entre_arreglo_y_object", sobreNuloArregloFuente, sobreNuloArregloJavac},
		{"arreglo_nuevo_a_object", sobreNuevoArregloFuente, sobreNuevoArregloJavac},
	}
	for _, c := range casos {
		nuestro, suyo := dexDeCaso(t, c.fuente, c.javac)
		if !bytes.Equal(nuestro, suyo) {
			t.Errorf("%s: el dex mide %d bytes y el de javac %d; el primero que difiere es el %d",
				c.nombre, len(nuestro), len(suyo), primerByteDistinto(nuestro, suyo))
		}
	}
}

// TestElNuloEntreDosRamasIncomparablesSeRechaza: `f(String)` y `f(Integer)` con `f(null)` es ambiguo
// para javac (`reference to f is ambiguous`), y aquí se rechaza en vez de elegir por el orden de la
// jerarquía. La fuente es la misma que la del rechazo medido con javac 17.0.21.
func TestElNuloEntreDosRamasIncomparablesSeRechaza(t *testing.T) {
	_, err := compilarProyectoJava([]fuenteJava{{nombre: "src/S.java", contenido: sobreNuloAmbiguoFuente}}, jarDelSDK(t))
	if err == nil {
		t.Fatal("se aceptó una llamada ambigua: `f(null)` con `f(String)` y `f(Integer)`")
	}
	if !strings.Contains(err.Error(), "mas especifica") {
		t.Errorf("el rechazo no explica que ninguna es la mas especifica: %v", err)
	}
}
