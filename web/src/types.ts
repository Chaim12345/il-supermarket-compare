export type UnitKind = 'kg' | 'unit';

export interface CatalogItem {
  id: string;
  he: string;
  en: string;
  category: string;
  unit: UnitKind;
  defaultQty: number;
  packLabel?: string;
}

export interface ChainInfo {
  key: string;
  name: string;
  kind: 'discount' | 'regular';
}

export interface Meta {
  catalog: CatalogItem[];
  chains: ChainInfo[];
  generatedAt: string;
  priceDate: string;
}

export interface LineResult {
  price: number;
  product: string;
  unit: string;
}

export interface StoreResult {
  storeId: string;
  chain: string;
  name: string;
  address: string;
  city: string;
  km: number;
  approx: boolean;
  total: number;
  found: number;
  lines: Record<string, LineResult>;
  missing: string[];
}

export interface LikeForLike {
  storeId: string;
  name: string;
  total: number;
}

export interface CompareResponse {
  error?: string;
  origin?: { lat: number; lon: number; label: string };
  date?: string;
  results: StoreResult[];
  summary?: {
    likeForLike: LikeForLike[];
    commonCount: number;
    cherryPick: number;
    cherryMissing: number;
  };
  skipped?: string[];
  radiusKm?: number;
}

export interface BasketLine {
  itemId: string;
  qty: number;
}
