-- Enable UUID generation
create extension if not exists "pgcrypto";

-- Pantry items (MVP)
create table if not exists public.pantry_items (
  id uuid primary key default gen_random_uuid(),
  user_id text not null,
  name text not null,
  quantity text,
  created_at timestamptz not null default now()
);

-- Shopping list (MVP)
create table if not exists public.shopping_list_items (
  id uuid primary key default gen_random_uuid(),
  user_id text not null,
  name text not null,
  is_checked boolean not null default false,
  created_at timestamptz not null default now()
);

-- Optional but recommended: cache LLM results
create table if not exists public.recipes_cache (
  id uuid primary key default gen_random_uuid(),
  pantry_hash text not null,
  preferences_hash text not null,
  response_json jsonb not null,
  created_at timestamptz not null default now(),
  unique (pantry_hash, preferences_hash)
);
