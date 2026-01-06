import { useState, useEffect, useCallback } from 'react';
import * as Api from '../lib/api';
import type { Config } from '../lib/api';

export function useSettings() {
  const [config, setConfig] = useState<Config | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  // Fetch config
  useEffect(() => {
    Api.GetConfig()
      .then(setConfig)
      .catch(console.error)
      .finally(() => setLoading(false));
  }, []);

  // Save config
  const saveConfig = useCallback(async (newConfig: Config) => {
    setSaving(true);
    try {
      await Api.SaveConfig(newConfig);
      setConfig(newConfig);
    } finally {
      setSaving(false);
    }
  }, []);

  // Update single field
  const updateField = useCallback(<K extends keyof Config>(
    field: K,
    value: Config[K]
  ) => {
    if (!config) return;
    setConfig({ ...config, [field]: value });
  }, [config]);

  return {
    config,
    loading,
    saving,
    saveConfig,
    updateField
  };
}
