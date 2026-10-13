-- +goose Up
CREATE TABLE installation_state (
 id TINYINT PRIMARY KEY, payload JSON NOT NULL
);
CREATE TABLE preset_versions (
 version INT PRIMARY KEY, payload JSON NOT NULL, created_at DATETIME(6) NOT NULL
);
CREATE TABLE products (
 id CHAR(32) PRIMARY KEY,
 sku VARCHAR(100) COLLATE utf8mb4_bin UNIQUE,
 name VARCHAR(200) NOT NULL, category VARCHAR(32) NOT NULL,
 cost DECIMAL(24,8) NOT NULL, price DECIMAL(24,8) NOT NULL,
 monthly_orders INT NOT NULL, fixed_costs DECIMAL(24,8) NOT NULL,
 packaging DECIMAL(24,8) NOT NULL, archived BOOLEAN NOT NULL DEFAULT FALSE,
 edit_version INT NOT NULL, payload JSON NOT NULL,
 created_at DATETIME(6) NOT NULL, updated_at DATETIME(6) NOT NULL,
 INDEX product_filters(category,archived,updated_at), INDEX product_names(name)
);
CREATE TABLE evaluation_revisions (
 id CHAR(32) PRIMARY KEY, product_id CHAR(32) NOT NULL,
 preset_version INT NOT NULL, calculation_version VARCHAR(32) NOT NULL,
 payload JSON NOT NULL, created_at DATETIME(6) NOT NULL,
 FOREIGN KEY(product_id) REFERENCES products(id),
 FOREIGN KEY(preset_version) REFERENCES preset_versions(version),
 INDEX evaluation_history(product_id,created_at)
);
CREATE TABLE competitors (
 id CHAR(32) PRIMARY KEY, product_id CHAR(32) NOT NULL, payload JSON NOT NULL,
 FOREIGN KEY(product_id) REFERENCES products(id)
);
CREATE TABLE product_images (
 id CHAR(32) PRIMARY KEY, product_id CHAR(32) NOT NULL, payload JSON NOT NULL,
 FOREIGN KEY(product_id) REFERENCES products(id)
);
CREATE TABLE jobs (
 id CHAR(32) PRIMARY KEY, kind VARCHAR(16) NOT NULL, status VARCHAR(16) NOT NULL,
 lease_until DATETIME(6) NOT NULL, payload JSON NOT NULL, created_at DATETIME(6) NOT NULL,
 INDEX job_queue(status,lease_until)
);
-- +goose Down
DROP TABLE jobs;
DROP TABLE product_images;
DROP TABLE competitors;
DROP TABLE evaluation_revisions;
DROP TABLE products;
DROP TABLE preset_versions;
DROP TABLE installation_state;
