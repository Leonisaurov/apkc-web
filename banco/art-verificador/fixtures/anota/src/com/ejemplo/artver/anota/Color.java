package com.ejemplo.artver.anota;

// El enum y la anotación del fixture, en tipos de primer nivel: ART tiene que poder leer los
// `encoded_annotation` que el dexer escribe para `valor_enum`, `valor_int` y `valor_string`, y
// `getGenericType` obliga además a leer la anotación `Signature` que deja javac.
public enum Color {
    ROJO, VERDE, AZUL
}
