create table users (id int primary key, email text, prefs jsonb);

insert into users (id, email, prefs) values
  (1, 'ada@example.com', '{"theme":"dark"}'),
  (2, 'alan@example.com', '{"theme":"light"}');
