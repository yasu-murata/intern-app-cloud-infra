import { useState, useEffect } from 'react';
import './App.css'; // 作成したCSSファイルをインポート

const ItemDialog = ({ item, onClose }) => {
  if (!item) return null;
  return (
    <div className="dialog-overlay" onClick={onClose}>
      <div className="dialog-content" onClick={(e) => e.stopPropagation()}>
        <h2>Item Details</h2>
        <p><strong>ID:</strong> {item.id}</p>
        <p><strong>Name:</strong> {item.name}</p>
        <p><strong>Description:</strong> {item.description}</p>
        <button onClick={onClose}>Close</button>
      </div>
    </div>
  );
};

function App() {
  const [items, setItems] = useState([]);
  const [selectedItem, setSelectedItem] = useState(null);
  const [error, setError] = useState(null);

  useEffect(() => {
    const fetchItems = async () => {
      try {
        const response = await fetch('/api/items');
        if (!response.ok) {
          throw new Error(`HTTP error! status: ${response.status}`);
        }
        const data = await response.json();
        setItems(data);
      } catch (e) {
        console.error("Failed to fetch items:", e);
        setError("Failed to load data. Please try again later.");
      }
    };
    fetchItems();
  }, []);

  const handleItemClick = async (id) => {
    try {
      const response = await fetch(`/api/items/${id}`);
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      const data = await response.json();
      setSelectedItem(data);
    } catch (e) {
      console.error(`Failed to fetch item ${id}:`, e);
      setError("Failed to load item details.");
    }
  };

  const closeDialog = () => {
    setSelectedItem(null);
  };

  return (
    <div className="App">
      <h1>Items List</h1>
      {error && <p className="error">{error}</p>}
      <table className="item-table">
        <thead>
          <tr>
            <th>ID</th>
            <th>Name</th>
          </tr>
        </thead>
        <tbody>
          {items.map((item) => (
            <tr key={item.id} onClick={() => handleItemClick(item.id)}>
              <td>{item.id}</td>
              <td>{item.name}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <ItemDialog item={selectedItem} onClose={closeDialog} />
    </div>
  );
}

export default App;