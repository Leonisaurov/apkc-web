package com.ejemplo.artver.anota;

import java.lang.annotation.ElementType;
import java.lang.annotation.Retention;
import java.lang.annotation.RetentionPolicy;
import java.lang.annotation.Target;

// Los tres elementos que cubren las formas del bloque de anotaciones: un `enum` (`value_enum`),
// un entero que aquí hace de `flags` (el formato no tiene un tipo aparte: va como `value_int`) y
// una cadena. Con `RetentionPolicy.RUNTIME` el bloque se queda en el dex y ART lo lee al pedirlo.
@Retention(RetentionPolicy.RUNTIME)
@Target({ElementType.TYPE, ElementType.FIELD, ElementType.METHOD})
public @interface Etiqueta {
    Color color() default Color.ROJO;
    int flags() default 0;
    String firma() default "";
}
