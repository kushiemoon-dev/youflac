import { useState, useEffect, useCallback } from 'react';
import * as App from '../../wailsjs/go/main/App';
import { backend } from '../../wailsjs/go/models';

export function useSettings() {
  const [config, setConfig] = useState<backend.Config | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  // Fetch config
  useEffect(() => {
    App.GetConfig()
      .then(setConfig)
      .catch(console.error)
      .finally(() => setLoading(false));
  }, []);

  // Save config
  const saveConfig = useCallback(async (newConfig: backend.Config) => {
    setSaving(true);
    try {
      await App.SaveConfig(newConfig);
      setConfig(newConfig);
    } finally {
      setSaving(false);
    }
  }, []);

  // Update single field
  const updateField = useCallback(<K extends keyof backend.Config>(
    field: K,
    value: backend.Config[K]
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
