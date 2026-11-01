package p;

// Un static de clase, una cuenta entera y un `if` con la condición invertida: lo que este fork emite
// y dexa hoy sin rechazar nada. Medido en el parrafo 61: nuestro emisor escribe `if_icmple 24` (al
// retorno), igual que javac, y `d8` lo baja a `if-le v1, v2` con el brazo `then` cayendo dentro del
// `if`, o sea con la misma inversión.
public class Ver {
    static int total = 5;

    public static void cuenta() {
        total = total + 2;
        if (total > 10) {
            total = total * 3;
        }
    }
}
