package com.wubuku.canvas.contract.reference.persistence;

import jakarta.persistence.Entity;
import jakarta.persistence.Id;
import jakarta.persistence.Lob;
import jakarta.persistence.Table;
import jakarta.persistence.Version;

@Entity
@Table(name = "reference_state")
public class ReferenceStateEntity {
    @Id
    private Long id;

    @Lob
    private String stateJson;

    @Version
    private long version;

    protected ReferenceStateEntity() {
    }

    public ReferenceStateEntity(Long id, String stateJson) {
        this.id = id;
        this.stateJson = stateJson;
    }

    public Long getId() {
        return id;
    }

    public String getStateJson() {
        return stateJson;
    }

    public void setStateJson(String stateJson) {
        this.stateJson = stateJson;
    }
}
