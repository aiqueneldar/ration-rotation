// ============================================
// API Service - Fetch Items from Backend
// ============================================
import type { Item } from "../App";

// Base URL for backend API (from environment variable)
const API_URL =
  import.meta.env.VITE_BACKEND_API_URL || "http://localhost:8080/api";

// ============================================
// Fetch Items List - Will call real API in Step 2
// For now, returns mock data to keep existing code working
// ============================================

export async function getItems(): Promise<Item[]> {
    const response = await fetch(`${API_URL}/items`);

    if (!response.ok) {
      throw new Error(`${response.statusText}`);
    }

    return await response.json();
}
