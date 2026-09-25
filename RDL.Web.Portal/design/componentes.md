# Inventario de componentes

Nombres iguales en diseño, notas y código. Todos: foco visible (`--focus-ring`), operables con teclado, estado nunca solo por color (icono + texto).

| Componente | Props / variantes | Estados | Notas |
|---|---|---|---|
| `Button` | `variant: primary \| secondary \| tertiary \| danger \| icon`, `size: sm(32) \| md(36) \| lg(44)`, `loading`, `disabled`, `kbd?` | normal, hover, foco, activo, deshabilitado, cargando (mantiene ancho, texto en gerundio: «Emitiendo…») | Radio 6. Texto 14/500. `lg` en móvil. |
| `ButtonGroup` | `items[]` | — | Descargas PDF / XML / Respuesta. |
| `ActionMenu` | `items[{label, danger?, disabled?}]` | abierto, opción activa | Acciones destructivas al final, separadas. |
| `TextField` | `label, help?, error?, required?, readOnly?` | normal, foco, error, deshabilitado, solo lectura | Etiqueta arriba, ayuda/error abajo. Error dice qué pasó y qué hacer. |
| `TextArea` | igual que `TextField`, `minLength?` | + contador de mínimo (motivos) | Motivos obligatorios: mínimo 10 caracteres. |
| `MoneyInput` | `currency: CRC \| USD`, `value`, `onChange(number)` | normal, foco, error | Prefijo de moneda visible, cifras tabulares, alineado a la derecha. Acepta «113 000,00», «113000», «₡113 000,00». Envía número con punto decimal. |
| `QuantityInput` | `decimals: 3`, `unit` | — | |
| `PercentInput` | `max: 100` | — | Descuento > 0 exige motivo. |
| `DateField` | `value dd/mm/aaaa`, `tz` | — | Muestra «Hora de Costa Rica». Siempre en zona de la organización. |
| `Select` | `options` | — | |
| `Combobox` | `search, options, onCreate?` | abierto, sin resultados, crear «x» | Clientes (con alta rápida), productos, CABYS (texto o 13 dígitos). |
| `Checkbox`, `Switch`, `Radio` | — | marcado, sin marcar, deshabilitado | |
| `FileUpload` | `accept='.p12'` | vacío, elegido, error | Certificado. PIN en campo aparte, nunca se muestra. |
| `DataTable` | `columns, rows, density: comfortable \| compact, selectable, expandable, stickyHeader, totalsRow, cursor{prev,next}` | cargando (esqueleto), vacío, error, fila activa, seleccionadas (barra de lote) | Numéricas a la derecha. Paginación por cursor: solo «Anterior / Siguiente». ↑ ↓ fila activa, Enter abre, Espacio selecciona, → ← expande. |
| `StatusBadge` | `domain: invoice \| hacienda \| receivable \| payment \| membership \| invitation`, `status`, `detail?`, `size: sm \| md` | — | Ver mapeo en `tokens` y `pantallas.md`. |
| `Tag` | `label` | — | Datos que no son estado (sucursal, condición). |
| `Timeline` | `events[{tone, icon, label, detail?, time?}]` | — | Historial de factura y de documento electrónico. |
| `KpiCard` | `label, value, sub?, tone?, link?` | cargando, error con reintentar, ok | Cada tarjeta carga y falla sola. |
| `AgingBar` / `AgingChart` | `buckets[{label, amount, color}]` | — | Cada tramo con nombre, monto y %. |
| `Tabs` | `items[{label, count?}]` | activa | Contadores por pestaña (bandeja). |
| `Breadcrumbs` | `items` | — | En listas y detalles «‹ Sección». |
| `Stepper` | `steps, current` | hecho, actual, pendiente | En móvil solo muestra el nombre del paso actual. |
| `Accordion` | — | abierto/cerrado | Intentos de envío. |
| `ConfirmDialog` | `title, summary[], warning, reasonRequired?, confirmLabel, tone` | idle, enviando, error (con reintento seguro) | Emitir: resumen. Anular/revertir/anular pago: motivo obligatorio. Cambiar a producción: escribir PRODUCCIÓN. |
| `Drawer` | `side: right`, `width: 440–480` | — | Alta rápida de cliente. |
| `Popover`, `Tooltip` | — | — | |
| `Toast` | `tone, title, body?, action?` | — | Abajo a la derecha; 5 s; los de error no se cierran solos. `aria-live=polite`. |
| `InlineAlert` | `tone, title?, body, actions?, refCode?` | — | Errores de servidor con código de referencia copiable. |
| `EmptyState`, `Skeleton`, `ErrorState` | — | — | |
| `SystemScreen` | `kind: 403 \| 404 \| unavailable \| unexpected \| session-expired` | — | 404 cubre «es de otra organización» sin revelarlo. |
| `AppShell` | `sidebarCollapsed` | — | Barra lateral 232/72 px (contraída: solo iconos + tooltip), barra superior 56 px. |
| `OrgSwitcher` | `orgs[{name, role}]`, `active` | abierto, con búsqueda | Confirma si hay cambios sin guardar. En móvil: hoja inferior. |
| `NotificationBell` | `unread, realtime: ok \| offline` | panel abierto, vacío, sin conexión (reconexión con cuenta regresiva) | En móvil: panel a pantalla completa. |
| `EnvBadge` | `env: test \| prod` | — | Siempre visible en la barra superior. |
| `ThemeToggle` | — | claro / oscuro | Cambia `data-theme` en la raíz. |
| `ShortcutSheet` | — | — | Se abre con `?`. |
| `TotalsPanel` | `totals, state: ok \| busy \| error, calculatedAt` | recalculando, error (no definitivos) | Los totales vienen del servidor; nunca se muestran como definitivos si falla el cálculo. |
| `PaymentAllocator` | `payment, openAccounts[]` | excedente (error), sobre saldo por fila, sin aplicar | Autollenado de la cuenta más antigua a la más nueva. Contador fijo Pago / Aplicado / Sin aplicar. |
