'use client';
import { useEffect, useState } from 'react';

type Row = { action: string; mac: string; other: string };
type Labels = { action: string; mac: string; other: string; platform: string };
type Platform = 'mac' | 'other';

export function ShortcutTable({ rows, labels }: { rows: Row[]; labels: Labels }) {
  const [platform, setPlatform] = useState<Platform>('mac');

  useEffect(() => {
    if (!/Mac|iPhone|iPad/.test(navigator.platform)) setPlatform('other');
  }, []);

  const tab = (value: Platform, text: string) => (
    <button
      type="button"
      role="tab"
      aria-selected={platform === value}
      onClick={() => setPlatform(value)}
      className="rounded-md px-3 py-1 text-sm font-medium text-fd-muted-foreground transition-colors hover:text-fd-foreground aria-selected:bg-fd-background aria-selected:text-fd-foreground aria-selected:shadow-sm"
    >
      {text}
    </button>
  );

  return (
    <div className="not-prose my-6">
      <div role="tablist" aria-label={labels.platform} className="inline-flex gap-1 rounded-lg bg-fd-muted p-1">
        {tab('mac', labels.mac)}
        {tab('other', labels.other)}
      </div>
      <div className="mt-3 overflow-hidden rounded-lg border">
        <table className="w-full text-sm">
          <thead className="bg-fd-muted/50 text-left">
            <tr>
              <th className="px-4 py-2 font-medium">{labels.action}</th>
              <th className="px-4 py-2 text-right font-medium">{platform === 'mac' ? labels.mac : labels.other}</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.action} className="border-t">
                <td className="px-4 py-2">{row.action}</td>
                <td className="px-4 py-2 text-right">
                  <span className="inline-flex gap-1">
                    {(platform === 'mac' ? row.mac : row.other).split('+').map((key, i) => (
                      <kbd key={i} className="rounded border bg-fd-muted px-1.5 py-0.5 font-mono text-xs">
                        {key}
                      </kbd>
                    ))}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
