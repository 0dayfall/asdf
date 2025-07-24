CREATE TABLE users (
  id SERIAL PRIMARY KEY,
  subject TEXT UNIQUE NOT NULL,
  aliases TEXT[],
  properties JSONB,
  links JSONB NOT NULL
);
