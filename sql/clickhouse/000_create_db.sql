-- Migration: 000_create_db
-- Description: Create database for ClickHouse tables
-- This must run before any other ClickHouse migrations

CREATE DATABASE IF NOT EXISTS solana_lab;
