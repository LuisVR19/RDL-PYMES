package app

// pageLimit aplica el tamaño por defecto y el máximo de convenciones §9.
func pageLimit(limit int) int {
	if limit <= 0 {
		return DefaultPageSize
	}
	return min(limit, MaxPageSize)
}

// cutPage recibe hasta limit+1 elementos (se pide uno de más para saber si hay otra página sin un COUNT) y
// devuelve la página y el cursor de la siguiente, o nil si no hay más.
func cutPage[T any](items []T, limit int, position func(T) PageCursor) ([]T, *PageCursor) {
	if len(items) <= limit {
		return items, nil
	}
	items = items[:limit]
	next := position(items[len(items)-1])
	return items, &next
}
