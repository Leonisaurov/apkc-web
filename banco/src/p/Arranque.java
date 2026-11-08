package p;

// El banco se ejecuta con `app_process` en el aparato, que exige un `main(String[])`. Desde el §66
// este fork **si** emite tipos de array, asi que el banco compila esta fuente de dos maneras: con
// javac (para el camino de la JVM y el de `d8`) y con `apkc compile` (el camino "todo nuestro", que
// corre el dex escrito por nosotros sin javac ni d8 de por medio). Aqui no hay `+` de cadenas: javac 21
// lo baja con `invokedynamic` y el dexer lo rechaza por nombre (plan 5.22).
public class Arranque {
    public static void main(String[] args) {
        Ver.cuenta();
        Ver.cuenta();
        Ver.suma(3);
        Ver.elige(1);
        Ver.cuenta();
        System.out.println(Ver.sumaArreglo(4));
        System.out.println(Ver.alMenosUna(3));
        System.out.println(Ver.primerIgual(5));
        System.out.println(Ver.sinDos(4));
        int[] corto = new int[2];
        corto[0] = 7;
        System.out.println(Ver.leeSeguro(corto, 0));
        System.out.println(Ver.leeSeguro(corto, 5));
        System.out.println(new Ver().privados());
        System.out.println(Ver.inicial());
        System.out.println(Ver.cuentaLarga(2L, 3L));
        System.out.println(Ver.acumula());
        System.out.println(Ver.finalmente(5));
        System.out.println(Ver.finalmente(0));
        System.out.println(Ver.salidaPendiente(5));
        System.out.println(Ver.salidaPendiente(0));
        System.out.println(Ver.flotantes(2));
        System.out.println(Ver.dobleCon(2));
        System.out.println(Ver.compara(2));
        System.out.println(new Ver().anchoPrivado());
        System.out.println(Ver.arreglosAnchos(2));
        char[] letra = new char[1];
        System.out.println(Ver.letras(letra, 2));
        System.out.println(Ver.clases());
        int t = Ver.total;
        System.exit(t);
    }
}
