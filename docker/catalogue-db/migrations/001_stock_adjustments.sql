-- SOCK-6: stock count constraints and the stock_adjustments audit table.
--
-- data/dump.sql already contains this schema, so a fresh catalogue-db container
-- needs nothing else. Run this file once against an existing socksdb that was
-- created from the older dump. Existing products keep their data; a NULL count
-- becomes 0. MySQL 5.7 parses the CHECK constraint but does not enforce it
-- (8.0.16+ does), so the service also refuses to take stock below zero.

UPDATE sock SET count = 0 WHERE count IS NULL;

ALTER TABLE sock
	MODIFY count int NOT NULL DEFAULT 0,
	ADD CHECK (count >= 0);

CREATE TABLE IF NOT EXISTS stock_adjustments (
	id INT NOT NULL AUTO_INCREMENT, 
	sock_id varchar(40) NOT NULL, 
	delta INT NOT NULL, 
	reason ENUM('sale', 'return', 'restock', 'correction') NOT NULL, 
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, 
	PRIMARY KEY(id), 
	INDEX idx_stock_adjustments_sock_created (sock_id, created_at DESC), 
	FOREIGN KEY (sock_id) 
		REFERENCES sock(sock_id)
);
