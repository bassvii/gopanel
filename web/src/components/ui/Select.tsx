import * as RadixSelect from '@radix-ui/react-select'
import { Check, ChevronDown, ChevronUp } from 'lucide-react'
import { forwardRef, type ReactNode } from 'react'

export interface SelectOption {
  value: string
  label: string
}

interface Props {
  value: string
  onChange: (value: string) => void
  options: SelectOption[]
  placeholder?: string
  disabled?: boolean
  className?: string
}

export const Select = forwardRef<HTMLButtonElement, Props>(function Select(
  { value, onChange, options, placeholder = 'Выберите...', disabled, className = '' },
  ref,
) {
  const selected = options.find((o) => o.value === value)

  return (
    <RadixSelect.Root value={value} onValueChange={onChange} disabled={disabled}>
      <RadixSelect.Trigger
        ref={ref}
        className={
          'inline-flex items-center justify-between gap-2 w-full rounded-lg px-3 py-2 text-sm ' +
          'outline-none transition-colors ' +
          className
        }
        style={{
          background: '#0a0a0a',
          border: '1px solid var(--border)',
          color: 'var(--text)',
        }}
        onFocus={(e) => {
          e.currentTarget.style.borderColor = 'var(--accent)'
        }}
        onBlur={(e) => {
          e.currentTarget.style.borderColor = 'var(--border)'
        }}
      >
        <RadixSelect.Value placeholder={placeholder}>
          {selected?.label}
        </RadixSelect.Value>
        <RadixSelect.Icon>
          <ChevronDown size={16} style={{ color: 'var(--text-dim)' }} />
        </RadixSelect.Icon>
      </RadixSelect.Trigger>

      <RadixSelect.Portal>
        <RadixSelect.Content
          position="popper"
          sideOffset={4}
          className="z-50 rounded-lg overflow-hidden"
          style={{
            background: 'var(--bg-surface)',
            border: '1px solid var(--border)',
            boxShadow: '0 10px 38px -10px rgba(0, 0, 0, 0.5)',
            minWidth: 'var(--radix-select-trigger-width)',
          }}
        >
          <RadixSelect.ScrollUpButton
            className="flex items-center justify-center h-6 cursor-default"
            style={{ color: 'var(--text-dim)' }}
          >
            <ChevronUp size={14} />
          </RadixSelect.ScrollUpButton>

          <RadixSelect.Viewport className="p-1">
            {options.map((o) => (
              <SelectItem key={o.value} value={o.value}>
                {o.label}
              </SelectItem>
            ))}
          </RadixSelect.Viewport>

          <RadixSelect.ScrollDownButton
            className="flex items-center justify-center h-6 cursor-default"
            style={{ color: 'var(--text-dim)' }}
          >
            <ChevronDown size={14} />
          </RadixSelect.ScrollDownButton>
        </RadixSelect.Content>
      </RadixSelect.Portal>
    </RadixSelect.Root>
  )
})

function SelectItem({ value, children }: { value: string; children: ReactNode }) {
  return (
    <RadixSelect.Item
      value={value}
      className="relative flex items-center gap-2 px-3 py-2 pr-8 rounded cursor-pointer text-sm outline-none select-none"
      style={{ color: 'var(--text)' }}
      onMouseEnter={(e) => {
        e.currentTarget.style.background = 'var(--bg-surface-hover)'
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.background = 'transparent'
      }}
      onFocus={(e) => {
        e.currentTarget.style.background = 'var(--bg-surface-hover)'
      }}
    >
      <RadixSelect.ItemText>{children}</RadixSelect.ItemText>
      <RadixSelect.ItemIndicator className="absolute right-2">
        <Check size={14} style={{ color: 'var(--accent)' }} />
      </RadixSelect.ItemIndicator>
    </RadixSelect.Item>
  )
}
