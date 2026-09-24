-- Endurecimiento de core.users, fase contract. Requiere desplegado el código que busca y da de alta usuarios
-- mediante core.find_user_by_subject / core.provision_user (00005). Sin users_platform, platform_app solo ve
-- su propia fila y las de los miembros de la organización activa, y no puede insertar ni borrar usuarios.

-- +goose Up
drop policy if exists users_platform on core.users;

-- +goose Down
create policy users_platform on core.users for all using (true) with check (true);
