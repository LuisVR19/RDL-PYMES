# Pantallas

Módulos de datos: **FAC** facturación · **HAC** Hacienda · **COB** cobranza · **ADM** administración.
Roles: O propietario · A administrador · F facturador · C cobrador · K contador · L solo lectura.
📱 = debe funcionar a 390 px. Lo no marcado: escritorio (1280–1440) y tableta (1024).
Estados comunes en listas: cargando (esqueleto), vacío (con acción), error (reintentar + código), sin permiso (403).

## A · Acceso y organización
| # | Pantalla | Ruta sugerida | Roles | Datos | Estados / notas |
|---|---|---|---|---|---|
| 1 📱 | Iniciar sesión / recuperar contraseña | `/ingresar`, `/recuperar` | público | auth | enviando, credenciales incorrectas (mensaje genérico), correo enviado (genérico), sesión vencida (vuelve a la ruta) |
| 2 📱 | Selector de organización | `/organizaciones` | usuario | ADM | cargando, sin organizaciones, error; búsqueda |
| 3 | Crear organización | `/organizaciones/nueva` | usuario | ADM | validación en línea, enviando, error del servidor (conserva lo escrito). Queda como propietario |
| 4 | Aceptar invitación | `/invitacion/:token` | invitado | ADM | válida, aceptando, vencida, revocada, ya usada, para otro correo |
| 5 | Cambio de organización | (barra superior) | todos | ADM | confirmación si hay cambios sin guardar; transición: atenuado 180 ms + esqueletos; toast con rol nuevo |

## B · Inicio
| 6 📱 | Inicio | `/` | todos | FAC, COB, HAC | cada bloque carga y falla solo; banner de datos parciales; organización nueva vacía. Paneles según rol |

## C · Facturación
| 7 | Clientes · lista | `/clientes` | O A F K L | FAC, COB (saldo) | filtro activos/inactivos, búsqueda |
| 8 | Cliente · crear/editar | `/clientes/nuevo`, `/clientes/:id/editar` | O A F | FAC | identificación duplicada (enlace al existente), error del servidor; aviso «editar no cambia facturas emitidas» |
| 9 📱 | Cliente · ficha | `/clientes/:id` | O A F K L | FAC, COB | pestañas Facturas / Cuentas por cobrar / Pagos / Datos; datos parciales si COB falla |
| 10 | Productos · lista | `/productos` | O A F K L | FAC | |
| 11 | Producto · crear/editar | `/productos/nuevo`, `/productos/:id/editar` | O A F | FAC | buscador CABYS (modal) |
| 12 | Documentos · lista | `/documentos` | O A F K L | FAC, HAC, COB | pestañas por tipo, densidad, fila con Hacienda no disponible sin romper la tabla |
| 13 | Factura · borrador | `/facturas/nueva`, `/facturas/:id/editar` | O A F | FAC | alta rápida de cliente (drawer), líneas de catálogo o libres, descuento exige motivo, totales del servidor (recalculando / error), validación antes de emitir, Ctrl S |
| 14 | Emitir | (diálogo) | O A F | FAC, HAC | confirmación con resumen; emitiendo; error con reintento seguro; emitida → detalle «En proceso» |
| 15 📱 | Factura · detalle | `/facturas/:id` | todos con FAC | FAC, HAC, COB | tres cifras (total, Hacienda, saldo); variantes aceptada, en proceso, rechazada (motivo + acción), contingencia, no disponible, anulada; solo lectura sin acciones |
| 16 | Nota de crédito / débito | `/facturas/:id/nota-credito`, `/nota-debito` | O A F | FAC | referencia fija; motivo obligatorio; débito con vencimiento |
| 17 | Anular factura | (diálogo) | O A F | FAC, COB | motivo obligatorio; explica ajuste de la cuenta por cobrar |

## D · Hacienda
| 18 | Configuración fiscal | `/hacienda/configuracion` | O A (K lectura) | HAC | ambiente muy visible (cambiar a producción: escribir PRODUCCIÓN); certificado vigente / por vencer / sin certificado; PIN nunca visible |
| 19 | Establecimientos y terminales | `/hacienda/establecimientos` | O A (K lectura) | HAC | códigos 3 y 5 dígitos, únicos, inmutables |
| 20 | Bandeja | `/hacienda/bandeja?estado=` | O A F K L | HAC | pestañas Rechazados / Contingencia / Con error / Todos; reintentar (contingencia y error) |
| 21 📱 | Documento electrónico | `/hacienda/documentos/:clave` | O A F K L | HAC | clave 50 y consecutivo 20 legibles y copiables sin espacios; intentos; archivos |

## E · Cobranza
| 22 | Cuentas por cobrar | `/cobranza/cuentas` | O A C K L | COB | filtros, días de atraso, totales por moneda |
| 23 | Aging | `/cobranza/aging?moneda=&corte=` | O A C K L | COB | tramos configurables; gráfico + tabla exportable |
| 24 📱 | Cuenta · detalle | `/cobranza/cuentas/:id` | O A C K L | COB, FAC | aplicaciones (revertir), ajustes, seguimientos y promesas con alta rápida |
| 25 | Pagos | `/cobranza/pagos` | O A C K L | COB | |
| 26 📱 | Registrar pago | `/cobranza/pagos/nuevo` | O A C | COB | stepper 3 pasos; excedente bloquea; puede quedar sin aplicar |
| 27 | Pago · detalle | `/cobranza/pagos/:id` | O A C K L | COB | revertir aplicación con motivo; anular pago (revierte todo) |

## F · Administración
| 28 | Organización | `/admin/organizacion` | O A | ADM | zona horaria, moneda por defecto |
| 29 | Sucursales | `/admin/organizacion/sucursales` | O A | ADM | código 3 dígitos único e inmutable; desactivar |
| 30 | Usuarios y roles | `/admin/usuarios` | O A | ADM | nunca sin propietario; solo propietario gestiona propietarios |
| 31 | Invitaciones | `/admin/usuarios/invitaciones` | O A | ADM | enlace visible una sola vez; revocar |
| 32 | Exportar auditoría | `/admin/auditoria` | O A | ADM | **depende de una API todavía no definida** |
| 33 📱 | Mi perfil | `/perfil` | todos | auth, ADM | |

## G · Transversales
| 34 | Notificaciones | (panel) | todos | eventos en tiempo real | contador, panel, toast, sin conexión con reconexión |
| 35 | Pantallas de sistema | `/403`, `/404`, `/error` | todos | — | sesión vencida en diálogo sin perder la ruta; código de referencia copiable |
| 36 | Atajos de teclado | (tecla `?`) | todos | — | `/` buscar · `N` nueva factura · `Ctrl S` guardar · `↑ ↓` filas · `Enter` abrir · `Espacio` seleccionar · `→ ←` expandir · `Esc` cerrar |

## Mapeo de estados (`StatusBadge`)
| Dominio | Valor | Etiqueta | Tono | Icono |
|---|---|---|---|---|
| Factura | draft / issued / cancelled | Borrador / Emitida / Anulada (tachado) | neutral / info / neutral | ○ ● ⊘ |
| Factura | requires_correction | Requiere corrección | warning | ▲ |
| Hacienda | processing, signed, sent | En proceso (+ detalle Firmando / Enviado) | info | ◷ |
| Hacienda | accepted / rejected / contingency / error / (sin dato) | Aceptada / Rechazada / En contingencia / Con error / Estado no disponible | success / danger / warning / danger / neutral | ✓ ✕ ▲ ! ? |
| Cuenta | open / partially_paid / paid / cancelled | Pendiente / Pago parcial / Pagada / Anulada | info / warning / success / neutral | ● ◐ ✓ ⊘ |
| Cuenta | vencida (atributo) | Vencida · N días | danger | ! |
| Pago | posted / voided | Registrado / Anulado | success / neutral | ✓ ⊘ |
| Membresía | active / suspended | Activo / Suspendido | success / neutral | ✓ ‖ |
| Invitación | pending / accepted / revoked / expired | Pendiente / Aceptada / Revocada / Vencida | info / success / neutral / neutral | ◷ ✓ ⊘ ○ |

Los glifos son provisionales; se propone Lucide (ISC) a 16/20 px, trazo 1,8.
