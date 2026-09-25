# Prompt · P8a Web Portal de la PYME · Diseño (para Claude Design)

> **Cómo usarlo**
> 1. Abre Claude Design y pega todo lo que está debajo de la línea. El prompt es autocontenido: Claude Design no ve los repositorios.
> 2. Pide primero el **sistema de diseño** (Paso 3) y apruébalo antes de las pantallas. Después ve por módulos, en el orden de la sección "Orden de trabajo".
> 3. Revisa cada módulo con alguien que conozca facturación electrónica en Costa Rica: los datos de ejemplo son ficticios y **ningún código fiscal debe tomarse como real**.
> 4. Al final, exporta el paquete de entrega (handoff) y guárdalo en `RDL.Web.Portal/design/`. Lo usa el prompt `P8b-web-portal-build.md`.

---

## Rol y objetivo

Eres un diseñador de producto senior especializado en **aplicaciones B2B de datos densos** (facturación, contabilidad, cobranza). Vas a diseñar el **portal web de la PYME** de un SaaS multiempresa de facturación electrónica para pequeñas y medianas empresas de **Costa Rica**.

Entregables:
1. un **sistema de diseño** completo (tokens, tipografía, componentes y patrones);
2. **todas las pantallas** del portal, con sus estados (cargando, vacío, error, sin permiso, datos parciales);
3. **prototipos navegables** de los flujos principales;
4. un **paquete de entrega** listo para que un desarrollador lo construya en React con TypeScript.

El portal lo usan personas que facturan todos los días y contadores que llevan varias empresas. Pesan más la **claridad, la velocidad de trabajo y la confianza** en los números que el efecto visual.

## El producto en una página

- **Organización:** la empresa (PYME) que contrata el servicio. Un usuario puede pertenecer a **varias** organizaciones (el caso típico es un contador) y trabaja siempre dentro de **una organización activa**, que puede cambiar.
- **Facturación (Billing):** clientes, productos y servicios (con código CABYS), facturas, notas de crédito y débito. Una factura nace como **borrador**, se **emite** (recibe número y queda fija) y se puede **anular** con motivo. Lo emitido nunca se edita.
- **Documento electrónico (Hacienda):** cada factura emitida genera un documento que se firma y se envía al Ministerio de Hacienda. Su estado es independiente del de la factura: una factura puede estar emitida y su documento **en proceso**, **aceptado**, **rechazado**, **en contingencia** (Hacienda no disponible) o **con error**.
- **Cobranza (Receivables):** cada factura emitida crea una **cuenta por cobrar** con saldo y vencimiento. Se registran **pagos**, y cada pago se **aplica** a una o varias cuentas (y una cuenta puede recibir varios pagos). Hay aging (antigüedad de saldos), seguimientos de cobro y promesas de pago.
- **Administración:** datos de la organización, sucursales, usuarios, roles e invitaciones.

La pantalla estrella es el **detalle de factura**, que junta en un solo lugar tres fuentes: el total (facturación), el estado ante Hacienda y el saldo pendiente (cobranza). Ejemplo:

```
Factura FAC-0000034
Total: ₡113 000,00  |  Hacienda: Aceptada  |  Saldo: ₡63 000,00
```

## Usuarios y roles

| Rol | Qué hace | Ve y usa |
|---|---|---|
| Propietario (`owner`) | Dueño de la cuenta | Todo, incluida la gestión de otros propietarios |
| Administrador (`admin`) | Administra la empresa | Todo, salvo gestionar propietarios |
| Facturador (`biller`) | Factura todo el día | Clientes, productos, facturas y notas; consulta de estado fiscal |
| Cobrador (`collector`) | Cobra | Cuentas por cobrar, pagos, aplicaciones, seguimientos y promesas |
| Contador (`accountant`) | Revisa, a veces en varias empresas | Consulta de todo, configuración fiscal (lectura) y reportes |
| Solo lectura (`read_only`) | Consulta | Consulta, sin acciones |

Reglas de diseño por rol:
- Lo que un rol **no puede hacer no se muestra** (menú y botones ocultos). Si el usuario llega por un enlace directo, ve una pantalla de "Sin permiso" clara, no un error técnico.
- Dibuja las pantallas clave en **dos variantes de rol** (por ejemplo, detalle de factura como administrador y como solo lectura) para que se vea qué desaparece.

## Idioma, formatos y datos

- **Español de Costa Rica**, trato de **usted**, tono profesional, claro y sin tecnicismos. Los errores dicen qué pasó y qué hacer ("La factura ya fue emitida; para corregirla cree una nota de crédito").
- **Moneda:** colones por defecto, con dólares posibles. Formato `₡113 000,00` y `US$1 250,00` (espacio como separador de miles, coma decimal). La moneda siempre visible junto al monto. Los montos van alineados a la derecha, con **cifras tabulares**.
- **Fechas:** `24/09/2026` y fecha con hora `24/09/2026 15:15`, siempre en la **zona horaria de la organización** (Costa Rica por defecto). Muestra la zona si hace falta aclarar ("hora de Costa Rica").
- **Identificaciones:** tipo + número (física, jurídica, DIMEX, NITE). Muestra el número tal cual se escribió.
- **Datos de ejemplo:** realistas pero **ficticios** (empresas y personas inventadas). Nunca uses empresas, cédulas ni códigos reales. Donde haga falta un código fiscal (CABYS, tarifa de impuesto, medio de pago), usa valores claramente de muestra y en las notas del diseño marca: "código ilustrativo, catálogo oficial pendiente".

## Paso 1 · Dirección visual

Propón **dos direcciones visuales** breves (paleta, tipografía, densidad y una pantalla de muestra, por ejemplo la lista de facturas) para elegir una. Condiciones:
- Sobria y confiable, de herramienta de trabajo; nada de estética de app de consumo ni ilustraciones protagonistas.
- **Densidad media-alta:** tablas con muchas filas legibles, formularios compactos. Ofrece una variante "cómoda" y otra "compacta" en las tablas.
- Tema **claro** como principal; tema **oscuro** definido en los tokens (no hace falta dibujar todas las pantallas en oscuro, sí el sistema y dos pantallas de muestra).
- Marca provisional **"RDL"**, con un logotipo tipográfico simple y fácil de reemplazar.

## Paso 2 · Principios de interacción

- **Escritorio primero** (1280–1440 px de ancho), usable en **tableta** (1024 px) y con las consultas y acciones rápidas usables en **móvil** (390 px): ver una factura, su estado y su saldo; registrar un pago; responder a una notificación. Los formularios largos (crear factura) pueden ser solo escritorio y tableta.
- **Accesibilidad WCAG 2.2 AA:** contraste suficiente, foco visible, todo operable con teclado, el color **nunca** es la única señal (cada estado lleva texto y un icono), áreas táctiles de al menos 44 px en móvil.
- **Teclado para quien factura mucho:** atajos para buscar (`/`), crear (`N`) y guardar, y navegación por filas de tabla con flechas. Documéntalos en una hoja de atajos.
- **Confirmación proporcional al riesgo:** emitir o anular pide confirmación con resumen; anular exige **motivo** escrito. Guardar un borrador no pide nada.
- **Nada se pierde:** un formulario con cambios sin guardar avisa antes de salir; un envío que falló conserva lo escrito y se puede reintentar.
- **Los números vienen del servidor:** los totales de una factura los calcula el sistema. Diseña el estado "recalculando" (un indicador junto a los totales) y el aviso si el cálculo falla, en vez de mostrar totales calculados en el navegador como definitivos.

## Paso 3 · Sistema de diseño (entregar y aprobar primero)

**Tokens** (con nombres pensados para convertirse en variables CSS, por ejemplo `--color-bg-surface`, `--space-4`, `--radius-md`):
- **Color por rol, no por tono:** fondo, superficie, superficie elevada, texto (principal, secundario, deshabilitado), borde, acento principal, foco, y los estados `success`, `warning`, `danger`, `info`, `neutral`, cada uno con variante de fondo suave y de texto.
- **Tipografía:** familia con cifras tabulares (por ejemplo Inter o similar, con licencia libre), escala de 6 a 8 tamaños, pesos, alturas de línea, y un estilo específico para montos.
- **Espaciado** en base 4, **radios**, **sombras** (pocas), **bordes**, **z-index**, **duraciones y curvas** de animación (sobrias; respeta "reducir movimiento").
- **Breakpoints** (móvil, tableta, escritorio, escritorio ancho).

**Colores de estado** (un mapeo único que usen todas las pantallas; cada uno con texto e icono):

| Dominio | Estado | Etiqueta en pantalla | Tono sugerido |
|---|---|---|---|
| Factura | `draft` | Borrador | neutral |
| Factura | `issued` | Emitida | info |
| Factura | `cancelled` | Anulada | neutral tachado o danger suave |
| Factura | marca `requires_correction` | Requiere corrección | warning |
| Hacienda | `processing`, `signed`, `sent` | En proceso (con detalle: firmando, enviado) | info |
| Hacienda | `accepted` | Aceptada | success |
| Hacienda | `rejected` | Rechazada | danger |
| Hacienda | `contingency` | En contingencia | warning |
| Hacienda | `error` | Con error | danger |
| Hacienda | sin dato | Estado no disponible | neutral con icono de aviso |
| Cuenta por cobrar | `open` | Pendiente | info |
| Cuenta por cobrar | `partially_paid` | Pago parcial | warning |
| Cuenta por cobrar | `paid` | Pagada | success |
| Cuenta por cobrar | `cancelled` | Anulada | neutral |
| Cuenta por cobrar | vencida (atributo, no estado) | Vencida · N días | danger |
| Pago | `posted` | Registrado | success |
| Pago | `voided` | Anulado | neutral |
| Membresía | `active` / `suspended` | Activo / Suspendido | success / neutral |
| Invitación | `pending`, `accepted`, `revoked`, `expired` | Pendiente, Aceptada, Revocada, Vencida | info, success, neutral, neutral |

**Componentes** (cada uno con variantes, tamaños y estados: normal, hover, foco, activo, deshabilitado, cargando, error):
- Botón (principal, secundario, terciario, peligro, solo icono), grupo de botones, menú de acciones.
- Campo de texto, área de texto, **campo de monto** (moneda visible, cifras tabulares, pegar "113 000,00" funciona), campo de cantidad, campo de porcentaje, **campo de fecha** (con la zona de la organización), selector, **combobox con búsqueda** (clientes, productos, CABYS), casilla, interruptor, radio, carga de archivo (certificado `.p12`).
- **Tabla de datos:** encabezado fijo, orden, filtros en barra, selección de filas, acciones por fila, fila expandible, columnas numéricas a la derecha, fila de totales, densidad cómoda y compacta, y **paginación por cursor** ("Anterior" y "Siguiente", sin "página 7 de 20": el total de páginas no se conoce).
- **Insignia de estado** (el mapeo de arriba), etiqueta, **línea de tiempo** de estados (para Hacienda y para el historial de la factura), **tarjeta de cifra** (KPI), barra de aging por tramos.
- Pestañas, migas de pan, **stepper** (crear factura, registrar pago), acordeón.
- Diálogo de confirmación (con motivo obligatorio cuando aplica), panel lateral (drawer), popover, tooltip.
- **Toast** y aviso en línea (info, éxito, advertencia, error). Los errores del servidor muestran un **código de referencia** copiable (para soporte).
- **Estado vacío** (con acción principal), **esqueleto de carga**, **estado de error** (con reintentar), **sin permiso**, **no encontrado**, **servicio no disponible**, **datos parciales** ("No pudimos obtener el estado de Hacienda; el resto de la información está al día").
- **Estructura de la aplicación:** barra lateral por módulos (colapsable), barra superior con **selector de organización activa**, buscador global, **campana de notificaciones** en tiempo real y menú de usuario.

## Paso 4 · Pantallas

Para **cada pantalla** entrega: propósito en una frase, quién la usa (roles), datos y acciones, **estados** (cargando, vacío, error, sin permiso, parcial cuando aplique) y la versión móvil cuando esté marcada con 📱.

### A. Acceso y organización

1. **Iniciar sesión** (correo y contraseña) y **recuperar contraseña**. Mensajes de error genéricos (no revelar si el correo existe). 📱
2. **Selector de organización activa**: lista de organizaciones del usuario con su rol en cada una; buscador si son muchas (contador). Estado sin organizaciones → invitación a crear una o a esperar una invitación. 📱
3. **Crear organización** (primera vez): razón social, nombre comercial, tipo y número de identificación, correo, teléfono y zona horaria. Quien la crea queda como propietario.
4. **Aceptar invitación** (desde un enlace): muestra la organización, el rol ofrecido y quién invitó; estados de invitación vencida, revocada o ya usada, y de "esta invitación es para otro correo".
5. **Cambio de organización:** desde la barra superior, con confirmación suave si hay un formulario sin guardar. Tras el cambio, toda la pantalla muestra datos de la nueva organización (dibuja la transición).

### B. Inicio

6. **Inicio / resumen** de la organización activa: facturado del mes, saldo por cobrar, saldo vencido, documentos rechazados o en contingencia (con enlace a la bandeja), últimas facturas y pagos. Cada bloque carga y falla de forma independiente (estado parcial). 📱

### C. Facturación

7. **Clientes · lista**: búsqueda por nombre o identificación, filtro activos o inactivos, saldo por cobrar por cliente. 
8. **Cliente · crear y editar**: identificación (tipo y número, única por organización), razón social, nombre comercial, correo, teléfono, dirección. Aviso de que editar un cliente **no cambia** las facturas ya emitidas.
9. **Cliente · ficha**: datos, facturas, cuentas por cobrar y saldo total, pagos. 📱
10. **Productos y servicios · lista**: código, descripción, CABYS, precio, moneda, impuesto, activo.
11. **Producto · crear y editar**: código, descripción, **buscador CABYS** (combobox o modal con búsqueda por texto y código de 13 dígitos), unidad de medida, precio, moneda, bien o servicio, impuestos.
12. **Documentos · lista** (facturas y notas): número, tipo, cliente, fecha, total, **estado de la factura**, **estado de Hacienda**, **saldo**, marca "requiere corrección". Filtros por tipo, estados, cliente y fechas. Una fila cuyo estado de Hacienda no se pudo obtener lo indica sin romper la tabla.
13. **Factura · crear y editar borrador**: cliente (combobox con alta rápida), sucursal, moneda y tipo de cambio, condición de venta y plazo, fecha de vencimiento, líneas (con producto del catálogo o línea libre; cantidad, precio, descuento con motivo obligatorio si hay descuento, impuesto), notas y **panel de totales** (subtotal, descuento, impuesto, exoneración, total) que recalcula el servidor. Validaciones en línea. Guardado de borrador.
14. **Factura · emitir**: diálogo de confirmación con resumen (cliente, total, moneda, vencimiento) y el aviso de que lo emitido no se edita; estados emitiendo, emitida (con el número asignado y el estado de Hacienda "En proceso") y error con reintento seguro.
15. **Factura · detalle (vista transversal)**: encabezado con número, cliente, fechas y las **tres cifras** (total, estado de Hacienda, saldo); líneas con impuestos y exoneraciones; totales; **línea de tiempo** de la factura y del documento electrónico; pagos aplicados; descargas (XML firmado, respuesta de Hacienda, PDF); acciones según estado y rol: emitir (si es borrador), anular con motivo, crear nota de crédito o débito, ver documento electrónico. Variantes: Hacienda **aceptada**, **rechazada** (con el motivo y la acción sugerida), **en contingencia** y **estado no disponible**. 📱
16. **Nota de crédito o débito · crear** desde una factura emitida: factura referenciada (fija), motivo obligatorio, líneas y totales; la de débito lleva vencimiento.
17. **Anular factura**: diálogo con motivo obligatorio y la consecuencia explicada (se ajusta la cuenta por cobrar).

### D. Hacienda y configuración fiscal

18. **Configuración fiscal**: perfil del contribuyente, **ambiente** (pruebas o producción, muy visible para no confundirlos), actividades económicas y **certificado de firma** (subir `.p12` con PIN; mostrar vigencia y aviso de vencimiento próximo; nunca mostrar el PIN).
19. **Establecimientos y terminales**: códigos (3 y 5 dígitos), relación con las sucursales.
20. **Bandeja de documentos electrónicos**: pestañas **Rechazados**, **En contingencia**, **Con error** y **Todos**; por fila, documento, cliente, motivo y antigüedad; acciones ver y reintentar (cuando el estado lo permite).
21. **Documento electrónico · detalle**: clave numérica (50 dígitos) y consecutivo (20 dígitos) legibles y copiables, línea de tiempo de estados, intentos de envío con respuesta de Hacienda, archivos y enlace a la factura. 📱

### E. Cobranza

22. **Cuentas por cobrar · lista**: documento, cliente, emisión, vencimiento, **días de atraso**, monto original, saldo, estado. Filtros por estado, cliente y vencidas; totales por moneda.
23. **Aging**: saldos por tramos (al día, 1–30, 31–60, 61–90, más de 90 días; tramos configurables), por moneda y por cliente, a una fecha de corte; gráfico de barras sobrio más tabla exportable.
24. **Cuenta por cobrar · detalle**: montos, saldo y estado; aplicaciones de pagos (con revertir), ajustes (notas, anulación), **seguimientos** (llamada, correo, visita, mensaje, nota) y **promesas de pago** (monto, fecha, estado), con alta rápida de ambos. 📱
25. **Pagos · lista**: fecha, cliente, monto, moneda, medio de pago, referencia, estado, monto aplicado y sin aplicar.
26. **Registrar pago** (stepper): 1) datos del pago (cliente, fecha, monto, moneda, medio, referencia); 2) **aplicar a cuentas**: tabla de las cuentas abiertas del cliente en la misma moneda, con saldo, donde se reparte el monto (autollenar de la más antigua a la más nueva, editar a mano), un contador de **aplicado / sin aplicar** siempre visible y error si lo aplicado supera el pago o el saldo de una cuenta; 3) confirmación. Un pago puede quedar sin aplicar. 📱 (versión simplificada)
27. **Pago · detalle**: aplicaciones (revertir con motivo), anular pago con motivo (explica que revierte sus aplicaciones).

### F. Administración

28. **Organización**: datos, zona horaria, moneda por defecto.
29. **Sucursales**: lista, crear, editar y desactivar (código único e inmutable).
30. **Usuarios y roles**: miembros con rol y estado, cambiar rol, suspender o reactivar; regla visible de que la organización no puede quedarse sin propietario (y que solo un propietario gestiona propietarios).
31. **Invitaciones**: invitar por correo con rol, pendientes, revocar, **copiar enlace** (hoy no se envía correo automático: el enlace se comparte a mano, y solo se muestra una vez al crearla).
32. **Exportar auditoría**: filtros por fecha, usuario y tipo de acción; descarga. Marca en el diseño "depende de una API todavía no definida".
33. **Mi perfil**: nombre, correo, organizaciones y cerrar sesión. 📱

### G. Transversales

34. **Notificaciones en tiempo real**: campana con contador y panel (documento aceptado, documento rechazado con acción, pago registrado, cuenta pagada); toast discreto al llegar; estado "sin conexión en tiempo real" con reconexión.
35. **Pantallas de sistema**: sin permiso (403), no encontrado (404, que también cubre "es de otra organización" sin revelarlo), sesión vencida (volver a iniciar sin perder la ruta), servicio no disponible y error inesperado con código de referencia.
36. **Hoja de atajos de teclado.**

## Paso 5 · Flujos para prototipar

Prototipos navegables, en escritorio salvo que se indique:
1. Iniciar sesión → elegir organización → inicio. (📱 también)
2. Crear cliente al vuelo dentro de una factura → agregar líneas con buscador CABYS → guardar borrador → emitir → detalle con Hacienda "En proceso" → llega la notificación "Aceptada".
3. Documento **rechazado**: notificación → bandeja → detalle con motivo → crear nota de crédito.
4. Registrar un pago y repartirlo entre tres cuentas, con el error de "aplicado mayor que el pago" y su corrección. (📱 versión simple)
5. Revertir una aplicación y anular un pago.
6. Invitar a un usuario como cobrador → el invitado acepta desde el enlace → entra con su rol (menú reducido).
7. Contador que cambia de organización y ve los datos de la otra empresa.

## Paso 6 · Entrega para desarrollo (handoff)

El diseño se construirá en **React con TypeScript** con un sistema de componentes propio y pequeño. Prepara la entrega así:
- **Tokens** exportables (nombres estables, valores en claro y oscuro) y documentados.
- **Inventario de componentes** con el mismo nombre en el diseño y en las notas (por ejemplo `StatusBadge`, `MoneyInput`, `DataTable`, `OrgSwitcher`), sus props o variantes y sus estados.
- Cada pantalla nombrada y numerada como en el Paso 4, con anotaciones de: ruta sugerida (por ejemplo `/facturas/:id`), roles que la ven, qué datos vienen de qué módulo (facturación, Hacienda, cobranza, administración), comportamiento responsive y estados.
- Textos definitivos de la interfaz (etiquetas, mensajes de error, estados vacíos) en una tabla, listos para un archivo de traducciones.
- Una lista de **decisiones abiertas y supuestos** (por ejemplo, los tramos del aging, las columnas de la exportación de auditoría, los códigos fiscales ilustrativos).

## Orden de trabajo

1. Dos direcciones visuales → elijo una.
2. Sistema de diseño completo (Paso 3) → apruebo.
3. Estructura de la aplicación y pantallas **A** (acceso) y **G** (transversales).
4. **C** (facturación), con el flujo 2.
5. **D** (Hacienda), con el flujo 3.
6. **E** (cobranza), con los flujos 4 y 5.
7. **B** (inicio) y **F** (administración), con los flujos 6 y 7.
8. Paquete de entrega.

Al terminar cada paso, muéstrame el resultado y **espera mi revisión** antes del siguiente.

## No hagas

- Inventar códigos fiscales, reglas de Hacienda o nombres de impuestos reales: usa marcadores ilustrativos y anótalos.
- Usar datos, cédulas o empresas reales.
- Mostrar totales calculados en el navegador como definitivos, o permitir editar algo emitido.
- Usar el color como única señal de un estado.
- Diseñar pantallas o acciones que no estén en esta lista sin anotarlas como propuesta.
