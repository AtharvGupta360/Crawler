import { useState } from 'react';

export default function TagsInput({ value = [], onChange, placeholder = 'Type and press Enter' }) {
  const [input, setInput] = useState('');

  const handleKeyDown = (e) => {
    if (e.key === 'Enter' && input.trim()) {
      e.preventDefault();
      const tag = input.trim();
      if (!value.includes(tag)) {
        onChange([...value, tag]);
      }
      setInput('');
    } else if (e.key === 'Backspace' && !input && value.length > 0) {
      onChange(value.slice(0, -1));
    }
  };

  const removeTag = (index) => {
    onChange(value.filter((_, i) => i !== index));
  };

  return (
    <div className="tags-input" onClick={(e) => e.currentTarget.querySelector('input')?.focus()}>
      {value.map((tag, i) => (
        <span key={i} className="tag">
          {tag}
          <button className="tag-remove" onClick={() => removeTag(i)}>×</button>
        </span>
      ))}
      <input
        value={input}
        onChange={(e) => setInput(e.target.value)}
        onKeyDown={handleKeyDown}
        placeholder={value.length === 0 ? placeholder : ''}
      />
    </div>
  );
}
