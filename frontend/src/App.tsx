import { useState, useMemo, useEffect } from "react";
import "./App.css";
import { useQuery } from "@tanstack/react-query";
import { getItems } from "./api/items";
import dayjs from "dayjs";
import localizedFormat from "dayjs/plugin/localizedFormat";
import relativeTime from "dayjs/plugin/relativeTime";
import sv from "dayjs/locale/sv"

// Extend Day.js with plugins
dayjs.extend(localizedFormat);
dayjs.extend(relativeTime);
// Set Day.js to use the user's locale
const userLocale = navigator.language || 'sv-SE'
console.log(`User locale set to: ${userLocale}`);

export interface Item {
  id: string;
  name: string;
  quantity: number;
  description?: string;
  ingredients?: string;
  stock_date?: string;
  best_before_date: string;
  days_before_warning?: number;
}

export interface ItemWithCalculated extends Item {
  daysUntilExpiry: number;
  status: "normal" | "warning" | "expired";
}

/* const items: Item[] = [
  {
    id: "d7021550-8b28-4d69-b3d3-2907e75461c2",
    name: "Milk",
    quantity: 2.5,
    stock_date: new Date().toISOString(),
    best_before_date: new Date("2026-10-25").toISOString(),
    days_before_warning: 7,
  },
  {
    id: "2c9de403-f327-4600-ad7d-49c9b23df5c2",
    name: "Rice",
    quantity: 1.0,
    stock_date: new Date().toISOString(),
    best_before_date: new Date("2026-12-01").toISOString(),
    days_before_warning: 7,
  },
  {
    id: "c1aecb72-190b-4a94-8204-c253187e2be4",
    name: "Coffee",
    quantity: 0.5,
    stock_date: new Date("2024-01-01").toISOString(),
    best_before_date: new Date("2024-01-01").toISOString(),
    days_before_warning: 7,
  },
  {
    id: "30416e45-ed16-4340-8a4c-4d8462b02133",
    name: "Beans",
    quantity: 3.0,
    stock_date: new Date().toISOString(),
    best_before_date: new Date("2026-11-01").toISOString(),
    days_before_warning: 7,
  },
];
*/

function getDaysUntilExpiry(bestBeforeDate?: string): number {
  if (!bestBeforeDate) return 0;
  const bestBefore = new Date(bestBeforeDate);
  const today = new Date();
  bestBefore.setUTCHours(0, 0, 0, 0);
  today.setUTCHours(0, 0, 0, 0);
  const diffTime = bestBefore.getTime() - today.getTime();
  const diffDays = Math.floor(diffTime / (1000 * 60 * 60 * 24));
  return diffDays;
}

function getExpireStatus(
  daysUntilExpiry: number,
  daysBeforeWarning?: number,
): "normal" | "warning" | "expired" {
  const warning = daysBeforeWarning ?? 7;
  if (daysUntilExpiry <= 0) return "expired";
  if (daysUntilExpiry > 0 && daysUntilExpiry <= warning) return "warning";
  return "normal";
}

function getStatusSymbol(status: string): string {
  switch (status) {
    case "normal":
      return "✔";
    case "warning":
      return "♻";
    case "expired":
      return "✖";
  }
  return "";
}

function getSortedItems(
  items: ItemWithCalculated[],
  config: { key?: keyof ItemWithCalculated; direction: "asc" | "desc" },
): ItemWithCalculated[] {
  if (!config.key) return items;

  const sortKey = config.key as keyof ItemWithCalculated;

  const sorted = [...items].sort((a, b) => {
    const aValue = a[sortKey];
    const bValue = b[sortKey];
    let comparison: number;
    const statusValues: Record<string, number> = {
      normal: 0,
      warning: -1,
      expired: -2,
    };
    // Special status sort as they have special meaning when sorting
    if (config.key === "status") {
      comparison =
        statusValues[aValue as string] - statusValues[bValue as string];
      // Then sort the rest numerically or lexiographically
    } else if (typeof aValue === "string" && typeof bValue === "string") {
      comparison = aValue.toLowerCase().localeCompare(bValue.toLowerCase());
    } else if (typeof aValue === "number" && typeof bValue === "number") {
      comparison = aValue - bValue;
    } else {
      comparison = 0;
    }
    return config.direction === "asc" ? comparison : -comparison;
  });
  return sorted;
}

const formatDate = (dateStr: string) => {
  return dayjs(dateStr).locale(userLocale).format("L");
};

// ============================================
// Main Component - ALL hooks here!
// ============================================

function App() {
  // Hook 1: Sort state
  const [sortConfig, setSortConfig] = useState<{
    key?: keyof ItemWithCalculated;
    direction: "asc" | "desc";
  }>({
    key: undefined,
    direction: "asc",
  });

  const itemsQuery = useQuery({
    queryKey: ["items"], // Cache key for invalidation
    queryFn: getItems, // Function that fetches data
  });

  // Destructure for clean access
  const { data: rawItems, isLoading, error } = itemsQuery;

  const itemsWithCalculations = useMemo(() => {
    if (!rawItems) return [];

    return rawItems.map((item) => ({
      ...item,
      daysUntilExpiry: getDaysUntilExpiry(item.best_before_date),
      status: getExpireStatus(
        getDaysUntilExpiry(item.best_before_date),
        item.days_before_warning ?? 7,
      ),
    }));
  }, [rawItems]); // Re-run when raw data changes

  const sortedItems = useMemo(() => {
    return getSortedItems(itemsWithCalculations, sortConfig);
  }, [itemsWithCalculations, sortConfig]);

  // Sort handler
  function handleSort(key: keyof ItemWithCalculated) {
    if (sortConfig.key === key) {
      setSortConfig((prev) => ({
        ...prev,
        direction: prev.direction === "asc" ? "desc" : "asc",
      }));
    } else {
      setSortConfig({
        key,
        direction: "asc",
      });
    }
  }

  if (isLoading) {
    return (
      <div className="app">
        <header className="header">
          <h1>📦 Ration Rotation Dashboard</h1>
          <p>Loading items from API...</p>
        </header>

        <main className="dashboard">
          <div className="empty-state">Loading inventory...</div>
        </main>

        <footer className="footer">
          <p>Please wait while we fetch your data.</p>
        </footer>
      </div>
    );
  } else if (error) {
    return (
      <div className="app">
        <header className="header">
          <h1>📦 Ration Rotation Dashboard</h1>
          <p style={{ color: "red" }}>Error: {(error as Error).message}</p>
        </header>

        <main className="dashboard">
          <div className="empty-state">Failed to load items</div>
        </main>

        <footer className="footer">
          <p>Data failed to load. Check your backend connection.</p>
        </footer>
      </div>
    );
  }

  return (
    <div className="app">
      <header className="header">
        <h1>📦 Ration Rotation Dashboard</h1>
        <p>{sortedItems.length} items tracked</p>
      </header>

      <main className="dashboard">
        {sortedItems.length === 0 ? (
          <div className="empty-state">No items to display</div>
        ) : (
          <table className="ration-table">
            <thead>
              <tr>
                <th
                  onClick={() => handleSort("status")}
                  style={{ cursor: "pointer" }}
                >
                  Status
                  {sortConfig.key === "status" && (
                    <span
                      className="sort-indicator"
                      data-direction={
                        sortConfig.key === "status"
                          ? sortConfig.direction
                          : undefined
                      }
                    >
                      {sortConfig.direction === "asc" ? "▲" : "▼"}
                    </span>
                  )}
                </th>

                <th
                  onClick={() => handleSort("name")}
                  style={{ cursor: "pointer" }}
                >
                  Item Name
                  {sortConfig.key === "name" && (
                    <span
                      className="sort-indicator"
                      data-direction={
                        sortConfig.key === "name"
                          ? sortConfig.direction
                          : undefined
                      }
                    >
                      {sortConfig.direction === "asc" ? "▲" : "▼"}
                    </span>
                  )}
                </th>

                <th
                  onClick={() => handleSort("quantity")}
                  style={{ cursor: "pointer" }}
                >
                  Quantity
                  {sortConfig.key === "quantity" && (
                    <span
                      className="sort-indicator"
                      data-direction={
                        sortConfig.key === "quantity"
                          ? sortConfig.direction
                          : undefined
                      }
                    >
                      {sortConfig.direction === "asc" ? "▲" : "▼"}
                    </span>
                  )}
                </th>

                <th
                  onClick={() => handleSort("best_before_date")}
                  style={{ cursor: "pointer" }}
                >
                  Best Before Date
                  {sortConfig.key === "best_before_date" && (
                    <span
                      className="sort-indicator"
                      data-direction={
                        sortConfig.key === "best_before_date"
                          ? sortConfig.direction
                          : undefined
                      }
                    >
                      {sortConfig.direction === "asc" ? "▲" : "▼"}
                    </span>
                  )}
                </th>

                <th
                  onClick={() => handleSort("daysUntilExpiry")}
                  style={{ cursor: "pointer" }}
                >
                  Expires
                  {sortConfig.key === "daysUntilExpiry" && (
                    <span
                      className="sort-indicator"
                      data-direction={
                        sortConfig.key === "daysUntilExpiry"
                          ? sortConfig.direction
                          : undefined
                      }
                    >
                      {sortConfig.direction === "asc" ? "▲" : "▼"}
                    </span>
                  )}
                </th>
              </tr>
            </thead>
            <tbody>
              {sortedItems.map((item) => (
                <tr key={item.id || item.name}>
                  <td>
                    <span
                      className={`status-icon ${item.status}`}
                      title={`${item.status.charAt(0).toUpperCase() + item.status.slice(1)} status`}
                    >
                      {getStatusSymbol(item.status)}
                    </span>
                  </td>
                  <td>{item.name}</td>
                  <td className="quantity">{item.quantity}</td>
                  <td className="date">
                    {item.best_before_date
                      ? formatDate(item.best_before_date)
                      : "No date"}
                  </td>
                  <td>{dayjs(item.best_before_date).fromNow()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </main>

      <footer className="footer">
        <p>{error ? "Connection failed." : "Data loaded from backend API!"}</p>
      </footer>
    </div>
  );
}

export default App;
