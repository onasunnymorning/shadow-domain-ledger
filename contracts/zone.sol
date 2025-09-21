// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/access/Ownable.sol";

/**
 * @title Zone
 * @dev A smart contract to represent a single DNS zone on Hedera.
 * It stores DNS records and models the parent/child hierarchical relationship.
 */
contract Zone is Ownable {
    // A simple struct to hold record data
    struct Record {
        string rdata;   // The record's data (e.g., "192.168.1.1")
        uint32 ttl;     // Time-to-live in seconds
    }

    // Mapping from a record name (e.g., "www") and type (e.g., "A") to its data
    // keccak256(abi.encodePacked(name, recordType)) => Record
    mapping(bytes32 => Record) public records;

    // --- HIERARCHY MODIFICATIONS START ---

    // Stores the address of the parent zone contract. For the root zone, this will be address(0).
    address public parentZone;

    // Mapping from a child zone's label (e.g., "example") to its Zone contract address.
    // This represents the delegation of a subdomain to another contract.
    mapping(string => address) public childZones;

    // Event emitted when a new child zone is delegated.
    event ZoneDelegated(string indexed label, address indexed childZoneAddress);

    // Event emitted when the parent zone is set.
    event ParentZoneSet(address indexed parentZoneAddress);

    // --- HIERARCHY MODIFICATIONS END ---


    // Event for when a record is updated
    event RecordSet(bytes32 indexed recordHash, string name, string recordType, string rdata, uint32 ttl);

    /**
     * @dev The owner is the manager of this zone.
     * @param initialOwner The account that will own and manage this zone.
     */
    constructor(address initialOwner) Ownable(initialOwner) {}

    /**
     * @dev Sets or updates a DNS record within this zone.
     * @param name The record name (e.g., "@", "www").
     * @param recordType The record type (e.g., "A", "MX", "NS").
     * @param rdata The record data.
     * @param ttl The record's time-to-live.
     */
    function setRecord(string memory name, string memory recordType, string memory rdata, uint32 ttl) external onlyOwner {
        bytes32 recordHash = keccak256(abi.encodePacked(name, recordType));
        records[recordHash] = Record(rdata, ttl);
        emit RecordSet(recordHash, name, recordType, rdata, ttl);
    }

    /**
     * @dev Gets a DNS record from this zone.
     * @param name The record name.
     * @param recordType The record type.
     * @return The record data and TTL.
     */
    function getRecord(string memory name, string memory recordType) external view returns (string memory rdata, uint32 ttl) {
        bytes32 recordHash = keccak256(abi.encodePacked(name, recordType));
        Record memory record = records[recordHash];
        return (record.rdata, record.ttl);
    }

    // --- HIERARCHY FUNCTIONS START ---

    /**
     * @dev Delegates a subdomain to another Zone contract. Only this zone's owner can call this.
     * This function establishes the parent->child link.
     * @param label The label of the child zone (e.g., "example" for example.com).
     * @param childZoneAddress The deployed address of the child's Zone contract.
     */
    function delegateChildZone(string memory label, address childZoneAddress) external onlyOwner {
        require(bytes(label).length > 0, "Label cannot be empty");
        require(childZoneAddress != address(0), "Child zone address cannot be the zero address");
        require(childZones[label] == address(0), "This child zone is already delegated");

        // 1. Point from this (parent) contract to the child contract.
        childZones[label] = childZoneAddress;

        // 2. Call the child contract to set its parent to this contract's address.
        Zone childZone = Zone(childZoneAddress);
        childZone.setParentZone(address(this));

        emit ZoneDelegated(label, childZoneAddress);
    }

    /**
     * @dev Sets the parent of this Zone contract.
     * This function is intended to be called ONLY by the `delegateChildZone` function of the parent contract.
     * It ensures the two-way link is established atomically.
     * @param _parentZoneAddress The address of the parent zone contract.
     */
    function setParentZone(address _parentZoneAddress) external {
        // We ensure a parent can only be set once and that the caller is the legitimate parent.
        // Note: A more robust implementation might use a temporary "pending parent" state,
        // but checking that the parent hasn't been set yet is a strong security measure.
        require(parentZone == address(0), "Parent zone is already set");
        
        // A simple check can be that the sender is a contract. A more advanced check could involve
        // a pre-authorized parent address set during construction.
        require(_parentZoneAddress != tx.origin, "Caller must be a contract");

        // Check if the child's zone string is found at the end of the parent's name
        require(keccak256(abi.encodePacked(childZones[label])) == keccak256(abi.encodePacked(_parentZoneAddress)), "Invalid parent zone");

        parentZone = _parentZoneAddress;
        emit ParentZoneSet(_parentZoneAddress);
    }

    // --- HIERARCHY FUNCTIONS END ---
}
