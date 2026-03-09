-- Migration: 000002_sync_tickets_id_sequence
-- Description: Sync tickets.id sequence with existing data before ticket numbers use it directly

SELECT setval(
    pg_get_serial_sequence('tickets', 'id'),
    COALESCE((SELECT MAX(id) FROM tickets), 1),
    EXISTS (SELECT 1 FROM tickets)
);
